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

import skvideo.io
import fdk
import numpy as np
import cv2 as cv
import requests
import ujson
import logging
import sys
import os


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


# todo: make this go in parallel to speed it up
def assemble_frames(video_path, list_of_image_urls, size, codec):
    video = skvideo.io.FFmpegWriter(video_path)

    for media_url in list_of_image_urls:
        resp = requests.get(media_url)
        resp.raise_for_status()
        img = cv.imdecode(
            np.array(bytearray(resp.content), dtype=np.uint8),
            cv.COLOR_GRAY2BGR
        )
        video.writeFrame(img)
    video.close()


# todo: make this go in parallel to speed it up
def delete_images(del_image_urls):
    for img_url in del_image_urls:
        requests.delete(img_url)


def handler(ctx, data=None, loop=None):
    log = get_logger(ctx)
    body = ujson.loads(data)
    get_urls = body.get("batch_object_get_urls", [])
    log.info("frames accepted: {0}".format(len(get_urls)))
    del_urls = body.get("batch_object_delete_urls", [])
    range_index = body.get("range_index", 0)
    log.info("range accepted: {0}".format(range_index))
    video_segment_url = body.get("video_segment_url")
    dimensions = body.get("dimensions")
    size = (dimensions.get("width"), dimensions.get("height"))

    # TODO: disable until newer base images with fully-functioning ffmpeg
    original_video_codec = "mp4v" # body.get("codec", "mp4v")

    log.info("incoming request parsed")
    video_path = "/tmp/{0}.mp4".format(range_index)
    assemble_frames(
        video_path,
        get_urls,
        size,
        original_video_codec,
    )
    log.info("video assembled")
    if int(os.environ.get("KEEP_IMAGES")) == 0:
        delete_images(del_urls)
        log.info("original images were deleted")

    with open(video_path, "rb") as videoFile:
        resp = requests.put(
            video_segment_url,
            data=videoFile.read(),
            headers={
                "Content-Type": "video/mp4",
            }
        )
        log.info("\n\trequest submitted to '{0}'"
                 "\n\tstatus code: '{1}'"
                 "\n\tresponse content: '{2}'".format(
                    video_segment_url,
                    resp.status_code,
                    resp.text))
        resp.raise_for_status()

    return "OK"


if __name__ == "__main__":
    fdk.handle(handler)
