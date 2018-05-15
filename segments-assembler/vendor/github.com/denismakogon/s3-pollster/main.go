package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/denismakogon/s3-pollster/api"
)

func start() error {

	ctx := context.Background()
	s3, err := api.NewFromEnv()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	return s3.DispatchObjects(ctx, wg)
}

func main() {
	err := start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
