package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/s3"
)

func generateKeysFromRangeNumber(rangeNumber int64) []string {
	var keys []string
	for i := int64(0); i < rangeNumber; i++ {
		keys = append(keys, fmt.Sprintf("%d.mp4", i))
	}
	return keys
}

func keyInRange(key string, keys []string) bool {
	for _, staleKey := range keys {
		if staleKey == key {
			return true
		}
	}
	return false
}

func allKeysInRange(objects []*s3.Object, keys []string) bool {
	for _, obj := range objects {
		if !keyInRange(*obj.Key, keys) {
			return false
		}
	}
	return true
}
