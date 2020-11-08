FROM golang:1.14-alpine AS builder
RUN apk add --no-cache gcc libffi-dev musl-dev

WORKDIR /go/src/app

# get deps
COPY go.mod go.sum ./
RUN go mod download

# build
COPY *.go ./
COPY internal ./internal
RUN go install .

FROM alpine:3.12
RUN apk add --no-cache exiftool>12.06-r0

COPY --from=builder /go/bin/ /app

WORKDIR /app
RUN mkdir ./data && touch ./data/configuration.yaml
COPY fonts ./fonts
COPY static ./static

EXPOSE 8080
CMD ["./photofield"]
