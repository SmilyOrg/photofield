###
# Client
###
FROM node:22-alpine AS node-builder
WORKDIR /ui

# install deps
COPY ui/package-lock.json ui/package.json ./
RUN npm install

# build
COPY ui .
RUN npm run build



###
# Server
###
FROM golang:1-alpine AS go-builder
# RUN apk add --no-cache gcc libffi-dev musl-dev libjpeg-turbo-dev

WORKDIR /go/src/app

# get deps
COPY go.mod go.sum ./
RUN go mod download

# build
COPY *.go ./
COPY defaults.yaml ./
COPY internal ./internal
COPY io ./io
COPY search ./search
COPY tag ./tag
COPY rangetree ./rangetree
COPY db ./db
COPY fonts ./fonts
COPY data/geo ./data/geo
# RUN go install -tags libjpeg .
COPY --from=node-builder /ui/dist/ ./ui/dist
RUN go install -tags embedui,embedgeo .



###
# Runtime
###
FROM alpine:latest
# RUN apk add --no-cache exiftool>12.06-r0 libjpeg-turbo
# libwebp enables high-performance native WebP encoding via the jackdyn encoder
# RUN apk add --no-cache exiftool ffmpeg libjpeg-turbo-utils
RUN apk add --no-cache exiftool ffmpeg libjpeg-turbo-utils libwebp && \
    ln -s /usr/lib/libwebp.so.7 /usr/lib/libwebp.so

WORKDIR /app

# RUN cp /usr/lib/libwebp.so.7 ./libwebp_amd64.so
# RUN cp /usr/lib/libwebp.so.7 ./libwebp.so
# RUN cp /usr/lib/libwebp.so.7 ./

COPY --from=go-builder /go/bin/ ./

RUN mkdir ./data && touch ./data/configuration.yaml

EXPOSE 8080
ENV PHOTOFIELD_DATA_DIR=./data
CMD ["./photofield"]
