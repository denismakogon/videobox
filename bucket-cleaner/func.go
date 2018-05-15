package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/denismakogon/s3-pollster/api"
	"github.com/fnproject/fdk-go"
	"github.com/sirupsen/logrus"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(withError))
}

func withError(ctx context.Context, in io.Reader, out io.Writer) {
	log := logrus.New()
	err := myHandler(ctx, in, out)
	if err != nil {
		fdk.WriteStatus(out, http.StatusInternalServerError)
		out.Write([]byte(err.Error()))
		log.Error(err.Error())
		return
	}
	fdk.WriteStatus(out, http.StatusAccepted)
}

func myHandler(_ context.Context, in io.Reader, _ io.Writer) error {
	var wg sync.WaitGroup
	var p RequestPayload
	err := json.NewDecoder(in).Decode(&p)
	if err != nil {
		return err
	}
	s, err := api.NewFromEndpoint(p.S3Endpoint)
	for {
		objs, err := s.Client.ListObjects(&s3.ListObjectsInput{
			Bucket: aws.String(p.Bucket),
		})
		if err != nil {
			return err
		}
		if len(objs.Contents) == 0 {
			break
		}

		wg.Add(len(objs.Contents))
		for _, obj := range objs.Contents {
			go func(wg *sync.WaitGroup, obj *s3.Object) {
				defer wg.Done()
				s.Client.DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(p.Bucket),
					Key:    obj.Key,
				})
			}(&wg, obj)
		}
	}

	wg.Wait()

	_, err = s.Client.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(p.Bucket),
	})
	return err
}
