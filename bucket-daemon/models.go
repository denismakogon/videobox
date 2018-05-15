package main

type RequestPayload struct {
	S3Endpoint        string           `json:"s3_endpoint"`
	Bucket            string           `json:"temp_bucket"`
	RangeNumber       int64            `json:"range_number"`
	OriginalObjectKey string           `json:"original_object_key"`
	Dimensions        map[string]int64 `json:"dimensions"`
	FramesPerSecond   int64            `json:"frames_per_second"`
}

type SegmentsAssemblerPayload struct {
	PreSignedURLs          []*string        `json:"pre_signed_urls"`
	Bucket                 string           `json:"bucket"`
	FinalVideoPreSignedURL string           `json:"final_video_pre_signed_url"`
	OriginalObjectKey      string           `json:"original_object_key"`
	Dimensions             map[string]int64 `json:"dimensions"`
	FramesPerSecond        int64            `json:"frames_per_second"`
	S3Endpoint             string           `json:"s3_endpoint"`
}
