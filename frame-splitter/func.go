package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"fmt"
	"os"

	"github.com/denismakogon/s3-pollster/api"
	"github.com/denismakogon/s3-pollster/common"
	"github.com/fnproject/fdk-go"
	"github.com/sirupsen/logrus"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(withError))
}

func withError(ctx context.Context, in io.Reader, out io.Writer) {
	err := myHandler(ctx, in)
	if err != nil {
		fdk.WriteStatus(out, http.StatusInternalServerError)
		out.Write([]byte(err.Error()))
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}
	fdk.WriteStatus(out, http.StatusAccepted)
}

func myHandler(ctx context.Context, in io.Reader) error {
	newVideo := &common.RequestPayload{}
	err := json.NewDecoder(in).Decode(newVideo)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(newVideo.Object, "test") {
		store, err := api.NewFromEndpoint(newVideo.S3Endpoint)
		if err != nil {
			return err
		}

		filePath, tempBucketName, err := SaveFile(ctx, store, newVideo)
		if err != nil {
			return err
		}

		logrus.Info("video saved, starting frame processing")
		err = SplitFrames(ctx, store, *filePath, *tempBucketName, newVideo.Object)
		if err != nil {
			os.RemoveAll(*filePath)
			return err
		}
	} else {
		logrus.Infof("skipping '%s' as it doesn't match pattern", newVideo.Object)
	}

	return nil
}
