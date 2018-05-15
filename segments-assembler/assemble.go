package main

import (
	"fmt"
	"log"

	"github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

func doAssemble(p *RequestPayload, frameMap [][]gocv.Mat) (*string, error) {
	fps := p.FramesPerSecond
	if fps == 0 {
		fps = 29
	}
	height := p.Dimensions["height"]
	if height == 0 {
		height = 780
	}
	width := p.Dimensions["width"]
	if width == 0 {
		width = 1240
	}

	fName := fmt.Sprintf("%s-%s.mp4", p.OriginalObjectKey, p.Bucket)
	logrus.Infof("Dimensions: %d(width) x %d(height)", width, height)
	finalVideo, err := gocv.VideoWriterFile(
		fName, "MP4V", float64(fps), int(width), int(height))
	if err != nil {
		return nil, err
	}
	defer finalVideo.Close()

	for _, videoFrames := range frameMap {
		for _, frame := range videoFrames {
			err := finalVideo.Write(frame)
			if err != nil {
				log.Fatal(err.Error())
			}
		}
	}

	return &fName, nil
}
