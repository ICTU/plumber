FROM golang:1.8
ARG APP_VERSION
WORKDIR /go/src/github.com/ICTU/plumber/
COPY ./ /go/src/github.com/ICTU/plumber/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-extldflags=-Wl,--allow-multiple-definition -X main.version=$APP_VERSION" -a -installsuffix cgo -o plumber *.go