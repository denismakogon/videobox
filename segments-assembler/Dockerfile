FROM denismakogon/gocv-alpine:3.4.2-buildstage as build-stage

LABEL maintainer="Denis Makogon. mail: lildee1991@gmail.com"
ADD . $GOPATH/src/function/
RUN cd $GOPATH/src/function/ && go build -o func

FROM denismakogon/gocv-alpine:3.4.2-runtime
COPY --from=build-stage /go/src/function/func /func
ENTRYPOINT ["/func"]
