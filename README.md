# VideoBox demo

## Involved functions

### Frame splitter

Operations:

 - parses a video into a chunks of 25 frames per second (this is a default value, might be more or less)
 - uploads chunks to temporary bucket
 - sets up pre-signed URLs for pulling and pushing frames to the temporary bucket
 - sets up pre-signed URLs for deleting frames from the temporary bucket
 - sets up pre-signed URL for assembled video frame from frames chunk
 - triggers a set of `object-detect` function (number of functions can be calculated by dividing total number of frames to FPS)
 - triggers `bucket-daemon` a recursive function that will watch on temporary bucket until requirement match.

Deployment command:

```bash
fn routes create videobox /frame-splitter --image $FN_REGISTRY/frame-splitter:0.0.34 --format json --type async --timeout 3600 --idle-timeout 10 --memory 1000
fn routes config set videobox /frame-splitter NEXT_FUNC /object-detect
```

### Object detection

Operations:

 - loops over a list of image URLs
 - downloads an image, does object detection and logo placement
 - puts an image back to store
 - triggers `segment-assembler` function

Deployment command:

```bash
fn routes create videobox /object-detect --image $FN_REGISTRY/object_detect:0.0.10 --format json --type async --timeout 3000 --idle-timeout 30 --memory 400
fn routes config set videobox /object_detect DETECT_PRECISION 0.4
fn routes config set videobox /object-detect NEXT_FUNC /segment-assembler
```


### Segment assembler

Operations:

 - pulls images in parallel
 - creates a video capture
 - uploads a video capture to the temporary bucket
 - deletes frames from temporary bucket

Deployment command:

```bash
fn routes create videobox /segment-assembler --image $FN_REGISTRY/segment-assembler:0.0.20 --format json --type async --timeout 30 --idle-timeout 20 --memory 400
```

### Bucket daemon

Experimental recursive function.

Operations:

 - watches the temporary bucket until requirement match
 - if requirement match, starts a `segments-assembler` function
 - quits with exit code `0`

Deployment command:

```bash
fn routes create videobox /bucket-daemon --image $FN_REGISTRY/bucket-daemon:0.0.4 --format json --type async --timeout 50 --idle-timeout 10 --memory 256
fn routes config set videobox /bucket-daemon NEXT_FUNC /segments-assembler
fn routes config set videobox /bucket-daemon BACKOFF_TIME 5
```


### Segments assembler

Operations:

 - reads video segments in parallel
 - reads frames from each video segment
 - writes frames back to original bucket with name prefix `test-{temporary-bucket-name}`

Deployment command:

```bash
fn routes create videobox /segments-assembler --image $FN_REGISTRY/segments-assembler:0.0.5 --format json --type async --timeout 3600 --idle-timeout 20 --memory 512
fn routes config set videobox /segments-assembler NEXT_FUNC /bucket-cleaner
```

### Bucket cleaner

Operations:

 - deletes bucket and its content

Deployment command:

```bash
fn routes create videobox /bucket-cleaner --image $FN_REGISTRY/bucket-cleaner:0.0.4 --format json --type async --timeout 360 --idle-timeout 10 --memory 256
```


## Deploy them all

```bash
fn -v deploy --all --registry `whoami`
```
