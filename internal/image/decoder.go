package image

import (
	"bytes"
	goimage "image"
	"image/jpeg"
	"io"
	"log"
	"photofield/tag"
	"strconv"
	"time"
)

type Decoder struct {
	loader       metadataLoader
	goexifLoader *GoExifRwcarlsenLoader
}

type metadataLoader interface {
	DecodeInfo(path string, info *Info) ([]tag.Tag, error)
	DecodeBytes(path string, tagName string) ([]byte, error)
	Close()
}

func NewDecoder(exifToolCount int) *Decoder {
	decoder := Decoder{}
	decoder.goexifLoader = NewGoExifRwcarlsenLoader()
	if exifToolCount > 0 {
		var err error
		decoder.loader, err = NewExifToolMostlyGeekLoader(exifToolCount)
		if err != nil {
			log.Printf("unable to use exiftool, defaulting to goexif - no video metadata support (%v)\n", err.Error())
			decoder.loader = decoder.goexifLoader
		}
	} else {
		decoder.loader = decoder.goexifLoader
	}
	return &decoder
}

func (decoder *Decoder) Close() {
	if decoder == nil {
		return
	}
	decoder.loader.Close()
}

func parseDateTime(value string) (t time.Time, hasTimezone bool, hasSubsec bool, err error) {
	t, err = time.Parse("2006:01:02 15:04:05Z07:00", value)
	if err == nil {
		hasTimezone = true
		hasSubsec = false
		return
	}
	t, err = time.Parse("2006:01:02 15:04:05", value)
	if err == nil {
		hasTimezone = false
		hasSubsec = false
		return
	}
	t, err = time.Parse("2006:01:02 15:04:05.99999999Z07:00", value)
	if err == nil {
		hasTimezone = true
		hasSubsec = true
		return
	}
	t, err = time.Parse("2006:01:02 15:04:05.99999999", value)
	if err == nil {
		hasTimezone = false
		hasSubsec = true
		return
	}
	return
}

func (decoder *Decoder) DecodeInfo(path string, info *Info) ([]tag.Tag, error) {
	return decoder.loader.DecodeInfo(path, info)
}

func (decoder *Decoder) DecodeImage(path string, tagName string) (goimage.Image, Info, error) {
	imageBytes, err := decoder.loader.DecodeBytes(path, tagName)
	if err != nil {
		return nil, Info{}, err
	}
	info := Info{}
	r := bytes.NewReader(imageBytes)
	decoder.goexifLoader.DecodeInfoReader(r, &info)

	r.Seek(0, io.SeekStart)
	img, err := jpeg.Decode(r)
	return img, info, err
}

func parseOrientation(orientation string) Orientation {
	n, err := strconv.Atoi(orientation)
	if err != nil || n < 1 || n > 8 {
		return Normal
	}
	return Orientation(n)
}

func getOrientationFromRotation(rotation string) Orientation {
	switch rotation {
	case "0":
		return Normal
	case "90":
		return Rotate90
	case "180":
		return Rotate180
	case "270":
		return Rotate270
	default:
		return Normal
	}
}
