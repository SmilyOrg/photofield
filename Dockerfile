# Build stage
FROM golang:1-alpine AS builder

ARG LDFLAGS=""

WORKDIR /src

# Build with bind mount (no COPY needed)
RUN \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=bind,source=.,target=/src \
  set -eou pipefail && \
  CGO_ENABLED=0 \
  go build \
    -ldflags "${LDFLAGS}" \
    -tags embedui,embeddocs,embedgeo \
    -o /build/photofield .

# Runtime stage
FROM alpine:3.22

RUN apk add --no-cache exiftool ffmpeg libjpeg-turbo-utils libwebp && \
    ln -s /usr/lib/libwebp.so.7 /usr/lib/libwebp.so

WORKDIR /app

COPY --from=builder /build/photofield ./photofield

RUN mkdir ./data && touch ./data/configuration.yaml

EXPOSE 8080
ENV PHOTOFIELD_DATA_DIR=./data
ENTRYPOINT ["./photofield"]
