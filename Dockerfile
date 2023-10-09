###
# Client
###
FROM node:18-alpine as node-builder
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
COPY db ./db
COPY fonts ./fonts
COPY data/geo ./data/geo
# RUN go install -tags libjpeg .
COPY --from=node-builder /ui/dist/ ./ui/dist
RUN go install -tags embedstatic,embedgeo .



###
# Runtime
###
FROM alpine:latest
# RUN apk add --no-cache exiftool>12.06-r0 libjpeg-turbo
RUN apk add --no-cache exiftool ffmpeg

COPY --from=go-builder /go/bin/ /app

WORKDIR /app
RUN mkdir ./data && touch ./data/configuration.yaml

EXPOSE 8080
ENV PHOTOFIELD_DATA_DIR=./data
CMD ["./photofield"]
