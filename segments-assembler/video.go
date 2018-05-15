package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/denismakogon/s3-pollster/api"
	"github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

func putVideoBack(fName string, p *RequestPayload) error {
	s, err := api.NewFromEndpoint(p.S3Endpoint)
	if err != nil {
		return err
	}
	videoFile, err := os.Open(fName)
	if err != nil {
		return err
	}
	defer videoFile.Close()
	_, err = s.Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(s.Config.Bucket),
		Key:         aws.String(fmt.Sprintf("test-%s%s", p.Bucket, ".mp4")),
		Body:        videoFile,
		ContentType: aws.String("video/mp4"),
	})
	return err
}

func videoFileFromURL(filePrefix int, mediaURL string) (*string, error) {
	resp, err := http.Get(mediaURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > http.StatusAccepted {
		return nil, fmt.Errorf("unable to get video "+
			"from pre-signed URL '%s'", mediaURL)
	}
	defer resp.Body.Close()
	tempFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("%d%s", filePrefix, ".mp4"))

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()
	fName := tempFile.Name()
	return &fName, nil
}

func videoToFrames(videoPath string) ([]gocv.Mat, error) {
	video, err := gocv.VideoCaptureFile(videoPath)
	if err != nil {
		return nil, err
	}
	logrus.Infof("video file '%s' opened", videoPath)
	return readFramesFromPosition(video, 0, int64(video.Get(gocv.VideoCaptureFrameCount)))
}

func readFramesFromPosition(video *gocv.VideoCapture, startFrameIndex, framesToRead int64) ([]gocv.Mat, error) {
	log := logrus.New()
	var mats []gocv.Mat
	tmpMat := gocv.NewMat()

	log.Infof("reading frames for range: [%d:%d]", startFrameIndex, startFrameIndex+framesToRead)
	for i := int64(0); i < framesToRead; i++ {
		success := video.Read(&tmpMat)
		if !success {
			return nil, fmt.Errorf("unable to read frame on position: '%d' "+
				"from a video file", startFrameIndex+i+1)
		}
		mats = append(mats, tmpMat)
	}
	width := tmpMat.Cols()
	height := tmpMat.Rows()
	log.Infof("Dimensions: %d(width) x %d(height)", width, height)
	defer video.Close()
	return mats, nil
}
