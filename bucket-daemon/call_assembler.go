package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/denismakogon/s3-pollster/api"
	"github.com/denismakogon/s3-pollster/common"
	"github.com/fnproject/fdk-go"
	"github.com/sirupsen/logrus"
)

func callAssembler(fctx *fdk.Ctx, store *api.Store, requestPayload *RequestPayload, keys []string, buf *bytes.Buffer) error {
	log := logrus.New()
	u, _ := url.Parse(fctx.RequestURL)
	fnAPIURL := fctx.RequestURL[:len(fctx.RequestURL)-len(u.EscapedPath())]

	log.Info("starting pre-signing HTTP Get URLs for video segments")
	preSignedURLs, err := getPreSignedURLsForVideoSegments(store, requestPayload.Bucket, keys)
	if err != nil {
		return err
	}
	log.Info("video segments HTTP Get URLs were pre-signed")
	finalVideoURL, err := putObjectPreSignedURL(
		store,
		aws.String("video/mp4"),
		aws.String(store.Config.Bucket),
		aws.String(fmt.Sprintf("test-%s%s", requestPayload.Bucket, ".mp4")))
	if err != nil {
		return err
	}

	log.Info("final video HTTP Put URL pre-signed")
	rP := SegmentsAssemblerPayload{
		PreSignedURLs:          preSignedURLs,
		Bucket:                 requestPayload.Bucket,
		FinalVideoPreSignedURL: *finalVideoURL,
		Dimensions:             requestPayload.Dimensions,
		FramesPerSecond:        requestPayload.FramesPerSecond,
		S3Endpoint:             requestPayload.S3Endpoint,
		OriginalObjectKey:      requestPayload.OriginalObjectKey,
	}
	err = json.NewEncoder(buf).Encode(rP)
	if err != nil {
		return err
	}
	json.NewEncoder(os.Stderr).Encode(rP)

	req, err := http.NewRequest(
		http.MethodPost, fmt.Sprintf("%s/r/%s%s",
			fnAPIURL, FnApp, NextFunc), buf)
	if err != nil {
		return err
	}

	err = common.DoRequest(req, http.DefaultClient)
	if err != nil {
		return err
	}

	log.Infof("request to '%s' submitted", NextFunc)
	return nil
}
