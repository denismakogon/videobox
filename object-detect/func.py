# All Rights Reserved.
#
#    Licensed under the Apache License, Version 2.0 (the "License"); you may
#    not use this file except in compliance with the License. You may obtain
#    a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
#    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
#    License for the specific language governing permissions and limitations
#    under the License.

import fdk
import numpy as np
import tensorflow as tf
import cv2 as cv
import requests
import ujson
import logging
import sys
import os

from PIL import Image
from urllib import parse


os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'
FN_PREFIX = "/function/tf_models"
FAST_TF_GRAPH = FN_PREFIX + "/frozen_inference_graph.pb"
FnLogo = FN_PREFIX + "/fn-logo.png"
LABEL_MAP = FN_PREFIX + "/coco_label_map.json"
FN_APP_NAME = os.environ.get("FN_APP_NAME")
DETECT_PRECISION = float(os.environ.get("DETECT_PRECISION", "0.3"))


def get_logger(ctx):
    root = logging.getLogger()
    root.setLevel(logging.INFO)
    ch = logging.StreamHandler(sys.stderr)
    call_id = ctx.CallID()
    formatter = logging.Formatter(
        '[call: {0}] - '.format(call_id) +
        '%(asctime)s - '
        '%(name)s - '
        '%(levelname)s - '
        '%(message)s'
    )
    ch.setFormatter(formatter)
    root.addHandler(ch)
    return root


def load_label_map():
    with open(LABEL_MAP, "r") as f:
        label_map = ujson.load(f)
        return label_map


def get_label_by_id(label_id, label_map):
    for m in label_map:
        _id = m.get("id")
        if _id is None:
            return {"display_name": "unknown"}
        if int(_id) == label_id:
            return m


def load_tf_graph():
    with tf.gfile.FastGFile(FAST_TF_GRAPH, 'rb') as f:
        graph_def = tf.GraphDef()
        graph_def.ParseFromString(f.read())
        tf.import_graph_def(graph_def, name='')
    return graph_def


def process_media(sess, media_url, log):
    resp = requests.get(media_url)
    resp.raise_for_status()

    img = cv.imdecode(
        np.array(bytearray(resp.content), dtype=np.uint8),
        cv.COLOR_GRAY2BGR
    )

    inp = cv.resize(img, (300, 300))
    inp = inp[:, :, [2, 1, 0]]  # BGR2RGB
    # Run the model
    out = sess.run(
        [sess.graph.get_tensor_by_name('num_detections:0'),
         sess.graph.get_tensor_by_name('detection_scores:0'),
         sess.graph.get_tensor_by_name('detection_boxes:0'),
         sess.graph.get_tensor_by_name('detection_classes:0')],
        feed_dict={'image_tensor:0':
                   inp.reshape(1, inp.shape[0], inp.shape[1], 3)})
    return img, out


def process_detection(out, img, label_map, detection_index, log):
    rows = img.shape[0]
    cols = img.shape[1]
    class_id = int(out[3][0][detection_index])
    label = get_label_by_id(class_id, label_map)
    score = float(out[1][0][detection_index])
    bbox = [float(v) for v in out[2][0][detection_index]]
    if score > DETECT_PRECISION:
        x = bbox[1] * cols
        y = bbox[0] * rows
        right = bbox[3] * cols
        bottom = bbox[2] * rows
        cv.rectangle(
            img,
            (int(x), int(y)),
            (int(right), int(bottom)),
            (125, 255, 51), thickness=2
        )
        cv.putText(
            img,
            label.get("display_name"),
            (int(x), int(y)+20),
            cv.FONT_HERSHEY_SIMPLEX,
            0.5, (255, 255, 255),
            2, cv.LINE_AA
        )

    return img


def post_image_back(img, frame):
    put_url = frame.get("put_object_url")
    resp = requests.put(
        put_url,
        data=np.array(cv.imencode('.jpg', img)[1]).tostring(),
        headers={
            "Content-Type": "image/jpeg",
        }
    )
    resp.raise_for_status()


def add_fn_logo(img):
    height, width, _ = img.shape
    img_pil = Image.fromarray(cv.cvtColor(img, cv.COLOR_BGR2RGB)).convert('RGB')
    mask = Image.open(FnLogo)
    mask_width, mask_height = mask.size
    img_ratio = float(height/width)
    mask_ratio = float(mask_height/mask_width)
    mask_scale_ration = 1
    if img_ratio > mask_ratio:
        # if original image is big - we need to scale up logo
        mask_scale_ration = img_ratio / mask_ratio
    if mask_ratio > img_ratio:
        # if original image is small - we need to scale down logo
        mask_scale_ration = mask_ratio / img_ratio

    custom_ratio = 3

    if 4 * mask_height > height:
        custom_ratio *= 1.5
    if 3 * mask_width > width:
        custom_ratio *= 2

    if height > 4 * mask_height:
        custom_ratio /= 1.5

    if width > 3 * mask_width:
        custom_ratio /= 2

    mask = mask.resize(
        (int(mask_width * mask_scale_ration / (custom_ratio * 4)),
         int(mask_height * mask_scale_ration / (custom_ratio * 4))), Image.ANTIALIAS)
    img_pil.paste(mask, (30, 30), mask=mask)

    return cv.cvtColor(np.array(img_pil), cv.COLOR_RGB2BGR)


def assemble_video_segment(ctx, fn_api_url, body):
    next_endpoint = "{0}/r/{1}{2}".format(
        fn_api_url, FN_APP_NAME,
        os.environ.get("NEXT_FUNC", "/segment-assembler"))
    resp = requests.post(next_endpoint, json=body)
    get_logger(ctx).info(resp.text)
    resp.raise_for_status()


def with_graph(label_map):

    sess = tf.Session()
    sess.graph.as_default()

    def fn(ctx, data=None, loop=None):
        u = parse.urlsplit(ctx.RequestURL())
        fn_api_url = u.scheme + "://" + u.netloc
        log = get_logger(ctx)
        frames = ujson.loads(data)
        object_get_collection = []
        object_del_collection = []
        range_index = frames.get("range_index", 0)
        video_segment_url = frames.get("video_segment_url")
        original_video_codec = frames.get("codec")
        height, width = 768, 1024
        for frame in frames.get("frames", []):
            media_url = frame.get("get_object_url")
            media_del_url = frame.get("del_object_url")

            img, out = process_media(sess, media_url, log)
            num_detections = int(out[0][0])

            for i in range(num_detections):
                img = process_detection(out, img, label_map, i, log)

            post_image_back(add_fn_logo(img), frame)

            object_get_collection.append(media_url)
            object_del_collection.append(media_del_url)
            height, width, _ = img.shape

        body = {
            "batch_object_delete_urls": object_del_collection,
            "batch_object_get_urls": object_get_collection,
            "range_index": range_index,
            "video_segment_url": video_segment_url,
            "dimensions": {
                "height": height,
                "width": width,
            },
            "codec": original_video_codec,
        }
        assemble_video_segment(ctx, fn_api_url, body)

    return fn


if __name__ == "__main__":
    load_tf_graph()
    fdk.handle(with_graph(load_label_map()))
