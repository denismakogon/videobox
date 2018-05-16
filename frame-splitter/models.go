package main

import "gocv.io/x/gocv"

type Input struct {
	Object string `json:"object"`
	Bucket string `json:"bucket"`
}

type Frame struct {
	GetObjectURL string `json:"get_object_url"`
	PutObjectURL string `json:"put_object_url"`
	DelObjectURL string `json:"del_object_url"`
}

type ObjectDetectPayload struct {
	Frames          []Frame `json:"frames"`
	Bucket          string  `json:"bucket"`
	Range           *Range  `json:"range"`
	RangeIndex      int64   `json:"range_index"`
	VideoSegmentURL string  `json:"video_segment_url"`
	Codec           string  `json:"codec"`
}

type Range struct {
	Start int64 `json:"start"`
	Stop  int64 `json:"stop"`
}

type RangedFrames struct {
	Frames []*gocv.Mat
	Range  *Range
}

type BucketDaemonPayload struct {
	S3Endpoint        string           `json:"s3_endpoint"`
	Bucket            string           `json:"temp_bucket"`
	RangeNumber       int64            `json:"range_number"`
	OriginalObjectKey string           `json:"original_object_key"`
	Dimensions        map[string]int64 `json:"dimensions"`
	FramesPerSecond   int64            `json:"frames_per_second"`
	Codec             string           `json:"codec"`
}
