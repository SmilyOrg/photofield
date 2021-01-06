###
# Server
###
FROM golang:1.14-alpine AS go-builder
RUN apk add --no-cache gcc libffi-dev musl-dev libjpeg-turbo-dev

WORKDIR /go/src/app

# get deps
COPY go.mod go.sum ./
RUN go mod download

# build
COPY *.go ./
COPY internal ./internal
# RUN go install -tags libjpeg .
RUN go install .



###
# Client
###
FROM node:14-alpine3.12 as node-builder
WORKDIR /ui

# install deps
COPY ui/package-lock.json ui/package.json ./
RUN npm install

# build
COPY ui .
RUN npm run build



###
# Runtime
###
FROM alpine:3.12
RUN apk add --no-cache exiftool>12.06-r0 libjpeg-turbo

COPY --from=go-builder /go/bin/ /app
COPY --from=node-builder /ui/dist/ /app/static

WORKDIR /app
RUN mkdir ./data && touch ./data/configuration.yaml
COPY fonts ./fonts

EXPOSE 8080
ENV API_PREFIX="/api"

CMD ["./photofield"]
