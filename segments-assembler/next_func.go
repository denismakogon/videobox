package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/denismakogon/s3-pollster/common"
	"github.com/fnproject/fdk-go"
	"github.com/sirupsen/logrus"
)

func callBucketCleaner(ctx context.Context, s3Endpoint, bucket string) error {
	log := logrus.New()
	fctx := fdk.Context(ctx)
	u, _ := url.Parse(fctx.RequestURL)
	fnAPIURL := fctx.RequestURL[:len(fctx.RequestURL)-len(u.EscapedPath())]

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(struct {
		S3Endpoint string `json:"s3_endpoint"`
		Bucket     string `json:"bucket"`
	}{
		S3Endpoint: s3Endpoint,
		Bucket:     bucket,
	})

	r, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/r/%s%s", fnAPIURL, FnApp, NextFunc), &buf)
	r.Header.Set("Content-Type", "application/json")
	if err != nil {
		return err
	}
	err = common.DoRequest(r, http.DefaultClient)
	log.Infof("request submitted to '%s'", NextFunc)
	return err
}
