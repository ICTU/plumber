FROM golang:1.8 as builder
ARG APP_VERSION
WORKDIR /go/src/github.com/ICTU/plumber/
COPY ./ /go/src/github.com/ICTU/plumber/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-extldflags=-Wl,--allow-multiple-definition -X main.version=$APP_VERSION" -a -installsuffix cgo -o plumber *.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /go/src/github.com/ICTU/plumber/plumber .
ENTRYPOINT ["/plumber"]
