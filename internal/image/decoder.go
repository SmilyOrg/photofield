package image

import (
	"bytes"
	goimage "image"
	"image/jpeg"
	"io"
	"log"
	"strconv"
	"time"
)

type Decoder struct {
	loader       metadataLoader
	goexifLoader *GoExifRwcarlsenLoader
}

type metadataLoader interface {
	DecodeInfo(path string, info *Info) error
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
	decoder.loader.Close()
}

func parseDateTime(value string) (time.Time, error) {
	t, err := time.Parse("2006:01:02 15:04:05Z07:00", value)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006:01:02 15:04:05", value)
	if err == nil {
		return t, nil
	}
	return t, err
}

func (decoder *Decoder) DecodeInfo(path string, info *Info) error {
	err := decoder.loader.DecodeInfo(path, info)
	// println(path, info.Width, info.Height, info.DateTime.String())
	// if info.Width != 0 {
	// 	println(path, info.String())
	// }
	return err
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
