package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/denismakogon/s3-pollster/common"
	"github.com/fnproject/fdk-go"
	"github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

var (
	FnApp    = os.Getenv("FN_APP_NAME")
	NextFunc = common.WithDefault("NEXT_FUNC", "/bucket-cleaner")
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

func myHandler(ctx context.Context, in io.Reader, _ io.Writer) error {
	log := logrus.New()
	var p RequestPayload
	err := json.NewDecoder(in).Decode(&p)
	if err != nil {
		return err
	}
	frameMap := make([][]gocv.Mat, len(p.PreSignedURLs))
	var wg sync.WaitGroup
	wg.Add(len(p.PreSignedURLs))

	fileMap := make(map[int]string, len(p.PreSignedURLs))

	for index, mediaURL := range p.PreSignedURLs {
		go func(wg *sync.WaitGroup, index int, mediaURL string) {
			videoFile, err := videoFileFromURL(index, mediaURL)
			defer wg.Done()
			if err != nil {
				log.Fatal(err.Error())
			}
			fileMap[index] = *videoFile
		}(&wg, index, mediaURL)
	}
	wg.Wait()

	for index, videoFile := range fileMap {
		frames, err := videoToFrames(videoFile)
		if err != nil {
			log.Fatal(err.Error())
		}

		frameMap[index] = frames
	}

	log.Info("all videos downloaded and parsed to frames")
	fName, err := doAssemble(&p, frameMap)
	if err != nil {
		return err
	}
	log.Info("final video assembled")

	err = putVideoBack(*fName, &p)
	if err != nil {
		return err
	}

	log.Info("final video submitted to store with response")
	return callBucketCleaner(ctx, p.S3Endpoint, p.Bucket)
}
