package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"net/url"
	"path/filepath"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/denismakogon/s3-pollster/api"
	"github.com/denismakogon/s3-pollster/common"
	"github.com/fnproject/fdk-go"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

var (
	FnAppName        = os.Getenv("FN_APP_NAME")
	NextFunc         = common.WithDefault("NEXT_FUNC", "/object-submit")
	BucketDaemonFunc = common.WithDefault("BUCKET_DAEMON_FUNC", "/bucket-daemon")
)

func SaveFile(ctx context.Context, s *api.Store, video *common.RequestPayload) (*string, *string, error) {

	prefix := uuid.New().String()
	tempFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("%s%s", prefix, filepath.Ext(video.Object)))
	if err != nil {
		return nil, nil, err
	}

	logrus.Info("saving file to: ", tempFile.Name())
	defer tempFile.Close()
	logrus.Info("setting up S3 connection")
	_, err = s.Downloader.DownloadWithContext(
		ctx, tempFile, &s3.GetObjectInput{
			Bucket: aws.String(video.Bucket),
			Key:    aws.String(video.Object),
		},
	)
	if err != nil {
		return nil, nil, err
	}

	logrus.Info("file was downloaded")
	tmpPath := tempFile.Name()
	return &tmpPath, &prefix, nil
}

func setupFrameInStore(ctx context.Context, s *api.Store, img image.Image,
	tempBucket string, frameIndex int64) (*string, *string, *string, error) {

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	contentType := aws.String("image/jpeg")
	bucket := aws.String(tempBucket)
	key := aws.String(fmt.Sprintf("%v.jpg", frameIndex))
	_, err = s.Uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      bucket,
		Key:         key,
		Body:        &buf,
		ContentType: contentType,
	})

	delObjectRequest, _ := s.Client.DeleteObjectRequest(&s3.DeleteObjectInput{
		Bucket: aws.String(tempBucket),
		Key:    key,
	})

	getObjectRequest, _ := s.Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(tempBucket),
		Key:    key,
	})
	putObjectRequest, _ := s.Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      bucket,
		Key:         key,
		ContentType: contentType,
	})

	delObjectRequestURL, err := delObjectRequest.Presign(1 * time.Hour)
	if err != nil {
		return nil, nil, nil, err
	}

	getObjectRequestURL, err := getObjectRequest.Presign(1 * time.Hour)
	if err != nil {
		return nil, nil, nil, err
	}

	putObjectRequestURL, err := putObjectRequest.Presign(1 * time.Hour)
	if err != nil {
		return nil, nil, nil, err
	}

	return &getObjectRequestURL, &putObjectRequestURL, &delObjectRequestURL, nil
}

func readFrameAndDispatch(ctx context.Context, s *api.Store,
	rangedFrames *RangedFrames, tempBucket string, rangeIndex int64, codec string) error {
	log := logrus.New()
	ff := &ObjectDetectPayload{}
	var tempFrames []Frame

	for i, frame := range rangedFrames.Frames {
		goImg, err := frame.ToImage()
		if err != nil {
			log.Fatal(err.Error())
		}
		frameIndex := rangedFrames.Range.Start + int64(i)
		getURL, putURL, delURL, err := setupFrameInStore(
			ctx, s, goImg, tempBucket, frameIndex)
		if err != nil {
			log.Fatal(err.Error())
		}

		f := Frame{
			GetObjectURL: *getURL,
			PutObjectURL: *putURL,
			DelObjectURL: *delURL,
		}
		tempFrames = append(tempFrames, f)
	}

	putVideoSegmentRequest, _ := s.Client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(tempBucket),
		Key:         aws.String(fmt.Sprintf("%d.mp4", rangeIndex)),
		ContentType: aws.String("video/mp4"),
	})

	putVideoSegmentURL, _ := putVideoSegmentRequest.Presign(1 * time.Hour)

	ff.VideoSegmentURL = putVideoSegmentURL
	ff.RangeIndex = rangeIndex
	ff.Frames = tempFrames
	ff.Bucket = tempBucket
	ff.Range = rangedFrames.Range
	ff.Codec = codec

	return callObjectDetect(ctx, ff)
}

func callObjectDetect(ctx context.Context, ff *ObjectDetectPayload) error {
	fctx := fdk.Context(ctx)
	u, _ := url.Parse(fctx.RequestURL)
	fnAPIURL := fctx.RequestURL[:len(fctx.RequestURL)-len(u.EscapedPath())]
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(ff)
	if err != nil {
		return err
	}

	req, _ := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/r/%s%s", fnAPIURL, FnAppName, NextFunc),
		&buf)
	err = common.DoRequest(req, http.DefaultClient)
	if err != nil {
		return err
	}
	logrus.Infof("request submitted to '%s'", NextFunc)
	return nil
}

func readFramesFromPosition(video *gocv.VideoCapture, startFrameIndex, framesToRead int64) []*gocv.Mat {
	log := logrus.New()
	var mats []*gocv.Mat
	tmpMat := gocv.NewMat()

	video.Set(gocv.VideoCapturePosFrames, float64(startFrameIndex))
	defer video.Set(gocv.VideoCapturePosFrames, float64(0))
	log.Infof("reading frames for range: [%d:%d]", startFrameIndex, startFrameIndex+framesToRead)

	for i := int64(0); i < framesToRead; i++ {
		success := video.Read(&tmpMat)
		if !success {
			log.Warningf("unable to read frame on position: '%d' "+
				"from a video file, maybe end of the video file?", startFrameIndex+i+1)
			return mats
		}
		mats = append(mats, &tmpMat)
	}
	return mats
}

func codecFromFourCC(video *gocv.VideoCapture) string {
	codecID := int64(video.Get(gocv.VideoCaptureFOURCC))
	res := ""
	hexes := []int64{0xff, 0xff00, 0xff0000, 0xff000000}
	for i, h := range hexes {
		res += string(codecID & h >> (uint(i * 8)))
	}
	return res
}

func SplitFrames(ctx context.Context, s *api.Store, filePath, tempBucket, originalObjectKey string) error {
	log := logrus.New()
	log.Info("reading video to OpenCV object")
	video, err := gocv.VideoCaptureFile(filePath)
	if err != nil {
		return err
	}
	fps := int64(video.Get(gocv.VideoCaptureFPS))
	if fps == 0 {
		// defaulting to 30 frames per seconds
		fps = 30
	}
	log.Info("Video has ", fps, " FPS")
	frameCount := int64(video.Get(gocv.VideoCaptureFrameCount))
	log.Info("Total frames found: ", frameCount)
	iterations := frameCount / fps
	log.Info("How many iterations to run: ", iterations)
	leftOvers := frameCount - iterations*fps
	log.Info("Frames left for the last iterations: ", leftOvers)

	if leftOvers > 0 {
		iterations += 1
	}

	videoHeight := int64(video.Get(gocv.VideoCaptureFrameHeight))
	videoWidth := int64(video.Get(gocv.VideoCaptureFrameWidth))

	log.Infof("video codec: %s", codecFromFourCC(video))
	log.Info("temporary bucket created: ", tempBucket)
	s.Client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(tempBucket),
	})

	log.Info("starting frame processing")
	var wg sync.WaitGroup
	wg.Add(int(iterations))

	for i := int64(0); i < iterations; i++ {
		stopLimit := (i + 1) * fps
		startLimit := i * fps
		if stopLimit > frameCount {
			stopLimit = frameCount
		}

		frames := readFramesFromPosition(video, startLimit, stopLimit-startLimit)
		if err != nil {
			return err
		}

		go func(wg *sync.WaitGroup, index int64, rfs *RangedFrames) {
			defer wg.Done()
			log.Infof("image posting to S3 round: %d with range: [%d:%d]",
				index, rfs.Range.Start, rfs.Range.Stop)
			err := readFrameAndDispatch(ctx, s, rfs, tempBucket, index, codecFromFourCC(video))
			if err != nil {
				log.Fatal(err)
			}
		}(&wg, i, &RangedFrames{
			Frames: frames,
			Range: &Range{
				Start: startLimit,
				Stop:  stopLimit,
			},
		})
	}

	wg.Wait()

	return callBucketDaemon(ctx, s.Config.RawEndpoint, tempBucket, originalObjectKey,
		iterations, fps, videoHeight, videoWidth, codecFromFourCC(video))
}

func callBucketDaemon(ctx context.Context, s3Endpoint, tempBucket, originalObjectKey string,
	rangeNumber, fps, videoHeight, videoWidth int64, codec string) error {

	bucketDaemonPayload := BucketDaemonPayload{
		Bucket:      tempBucket,
		S3Endpoint:  s3Endpoint,
		RangeNumber: rangeNumber,
		Dimensions: map[string]int64{
			"height": videoHeight,
			"width":  videoWidth,
		},
		FramesPerSecond:   fps,
		OriginalObjectKey: originalObjectKey,
		Codec:             codec,
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(bucketDaemonPayload)
	if err != nil {
		return err
	}
	fctx := fdk.Context(ctx)
	u, _ := url.Parse(fctx.RequestURL)
	fnAPIURL := fctx.RequestURL[:len(fctx.RequestURL)-len(u.EscapedPath())]

	r, _ := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/r/%s%s", fnAPIURL, FnAppName, BucketDaemonFunc),
		&buf,
	)

	return common.DoRequest(r, http.DefaultClient)
}
