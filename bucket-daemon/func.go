package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/denismakogon/s3-pollster/api"
	"github.com/denismakogon/s3-pollster/common"
	"github.com/fnproject/fdk-go"
	"github.com/sirupsen/logrus"
)

var (
	FnApp    = os.Getenv("FN_APP_NAME")
	NextFunc = common.WithDefault("NEXT_FUNC", "/segments-assembler")
	Backoff  = common.WithDefault("BACKOFF_TIME", "5")
)

func main() {
	fdk.Handle(fdk.HandlerFunc(withError))
}

func withError(ctx context.Context, in io.Reader, out io.Writer) {
	err := myHandler(ctx, in, out)
	if err != nil {
		fdk.WriteStatus(out, http.StatusInternalServerError)
		out.Write([]byte(err.Error()))
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}
	fdk.WriteStatus(out, http.StatusAccepted)
}

func myHandler(ctx context.Context, in io.Reader, _ io.Writer) error {
	log := logrus.New()
	fctx := fdk.Context(ctx)

	var p RequestPayload
	err := json.NewDecoder(in).Decode(&p)
	if err != nil {
		return err
	}

	s, err := api.NewFromEndpoint(p.S3Endpoint)
	if err != nil {
		return err
	}

	keys := generateKeysFromRangeNumber(p.RangeNumber)
	log.Infof("generated keys: %v", keys)
	// todo: make this object list more solid...
	objects, err := s.Client.ListObjects(&s3.ListObjectsInput{
		Bucket:  aws.String(p.Bucket),
		MaxKeys: aws.Int64(p.RangeNumber * 2),
	})
	if err != nil {
		return err
	}
	log.Infof("objects found: %d", len(objects.Contents))
	allIn := allKeysInRange(objects.Contents, keys)
	var wg sync.WaitGroup
	var buf bytes.Buffer
	if !allIn {
		wg.Add(1)
		go func(wg *sync.WaitGroup) {
			log.Info("requirement didn't matched, calling itself...")
			defer wg.Done()
			buf.Reset()
			err = json.NewEncoder(&buf).Encode(p)
			if err != nil {
				log.Fatal(err.Error())
			}
			t, _ := strconv.Atoi(Backoff)
			time.Sleep(time.Duration(t) * time.Second)
			req, _ := http.NewRequest(http.MethodPost, fctx.RequestURL, &buf)
			err = common.DoRequest(req, http.DefaultClient)
			if err != nil {
				log.Error(err.Error())
			}
			time.Sleep(time.Duration(t) * time.Second)
		}(&wg)
	} else {
		log.Info("requirement matched, aborting recursive function...")
		err := callAssembler(fctx, s, &p, keys, &buf)
		if err != nil {
			return err
		}
		os.Exit(0)
	}

	wg.Wait()

	return nil
}
