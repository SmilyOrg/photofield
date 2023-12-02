package exiftool

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"photofield/io"
	"time"

	"image/jpeg"

	"github.com/mostlygeek/go-exiftool"
)

type Exif struct {
	Tag      string `json:"tag"`
	exifTool *exiftool.Pool
}

func New(tag string) *Exif {
	e := Exif{
		Tag: tag,
	}
	exifTool, err := exiftool.NewPool(
		"exiftool", 4,
		"-n", // Machine-readable values
		"-S", // Short tag names with no padding
	)
	e.exifTool = exifTool
	if err != nil {
		panic(err)
	}
	return &e
}

func (e Exif) Close() error {
	e.exifTool.Stop()
	return nil
}

func (e Exif) Name() string {
	return fmt.Sprintf("exiftool-%s", e.Tag)
}

func (e Exif) Size(size io.Size) io.Size {
	return io.Size{X: 120, Y: 120}.Fit(size, io.FitInside)
}

func (e Exif) GetDurationEstimate(size io.Size) time.Duration {
	return 17 * time.Millisecond
}

func (e Exif) Get(ctx context.Context, id io.ImageId, path string) (image.Image, error) {
	b, err := e.decodeBytes(path)
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(b)
	return jpeg.Decode(r)
}

func (e Exif) Set(ctx context.Context, id io.ImageId, path string, img image.Image, err error) bool {
	return false
}

func (e Exif) decodeBytes(path string) ([]byte, error) {
	return e.exifTool.ExtractFlags(path, "-b", "-"+e.Tag)
}
