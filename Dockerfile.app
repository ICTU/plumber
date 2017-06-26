FROM alpine:latest
WORKDIR /
COPY plumber /plumber
ENTRYPOINT ["/plumber"]