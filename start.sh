#!/usr/bin/env bash

docker run -ti --rm --name s3-pollster \
    -e S3_URL="s3://admin:password@${S3_API_URL}/us-east-1/${S3_BUCKET:-videobox}" \
    -e WEBHOOK_ENDPOINT="${FN_API_URL}/r/videobox/frame-splitter" \
    -e POLLSTER_BACKOFF=5 \
    denismakogon/s3-pollster:0.0.3
