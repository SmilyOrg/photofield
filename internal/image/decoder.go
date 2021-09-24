package image

import (
	"log"
	"strconv"
	"time"
)

type Decoder struct {
	loader metadataLoader
}

type metadataLoader interface {
	DecodeInfo(path string, info *Info) error
	Close()
}

func NewDecoder(exifToolCount int) *Decoder {
	decoder := Decoder{}
	if exifToolCount > 0 {
		var err error
		decoder.loader, err = NewExifToolMostlyGeekLoader(exifToolCount)
		if err != nil {
			log.Printf("unable to use exiftool, defaulting to goexif - no video metadata support (%v)\n", err.Error())
			decoder.loader = NewGoExifRwcarlsenLoader()
		}
	} else {
		decoder.loader = NewGoExifRwcarlsenLoader()
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
