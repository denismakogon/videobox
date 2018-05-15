package main

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/denismakogon/s3-pollster/api"
)

func getPreSignedURLsForVideoSegments(store *api.Store, bucket string, keys []string) ([]*string, error) {
	var urls []*string

	for _, key := range keys {
		u, err := getObjectPreSignedURL(store, bucket, key)
		if err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}

	return urls, nil
}

func getObjectPreSignedURL(store *api.Store, bucket, objectKey string) (*string, error) {
	r, _ := store.Client.GetObjectRequest(&s3.GetObjectInput{
		Key:    aws.String(objectKey),
		Bucket: aws.String(bucket),
	})
	preSigned, err := r.Presign(1 * time.Hour)
	return &preSigned, err
}

func putObjectPreSignedURL(store *api.Store, contentType, bucket, objectKey *string) (*string, error) {
	r, _ := store.Client.PutObjectRequest(&s3.PutObjectInput{
		Key:         objectKey,
		Bucket:      bucket,
		ContentType: contentType,
	})
	preSigned, err := r.Presign(1 * time.Hour)
	return &preSigned, err
}
