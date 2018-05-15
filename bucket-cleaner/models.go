package main

type RequestPayload struct {
	S3Endpoint string `json:"s3_endpoint"`
	Bucket     string `json:"bucket"`
}
