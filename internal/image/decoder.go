package image

import (
	"image"
	"io"
	"strconv"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
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
		decoder.loader = NewExifToolMostlyGeekLoader(exifToolCount)
	} else {
		decoder.loader = NewGoExifRwcarlsenLoader()
	}
	return &decoder
}

func (decoder *Decoder) Close() {
	decoder.loader.Close()
}

func (decoder *Decoder) PostProcess(reader io.ReadSeeker, img image.Image) (image.Image, error) {
	_, err := reader.Seek(0, io.SeekStart)
	if err != nil {
		return img, err
	}
	orientation := getOrientation(reader)
	switch orientation {
	case "1":
	case "2":
		img = imaging.FlipH(img)
	case "3":
		img = imaging.Rotate180(img)
	case "4":
		img = imaging.Rotate180(imaging.FlipH(img))
	case "5":
		img = imaging.Rotate270(imaging.FlipV(img))
	case "6":
		img = imaging.Rotate270(img)
	case "7":
		img = imaging.Rotate90(imaging.FlipV(img))
	case "8":
		img = imaging.Rotate90(img)
	}
	return img, nil
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
	return err
}

func getRotationDimensionSwap(rotation string) bool {
	switch rotation {
	case "90":
		fallthrough
	case "270":
		return true
	default:
		return false
	}
}

func getOrientationDimensionSwap(orientation string) bool {
	switch orientation {
	case "1":
		return false
	case "2":
		return false
	case "3":
		return false
	case "4":
		return false
	case "5":
		return true
	case "6":
		return true
	case "7":
		return true
	case "8":
		return true
	default:
		return false
	}
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

func getOrientationFromExif(x *exif.Exif) string {
	if x == nil {
		return "1"
	}
	orient, err := x.Get(exif.Orientation)
	if err != nil {
		return "1"
	}
	if orient != nil {
		return orient.String()
	}
	return "1"
}

func (decoder *Decoder) DecodeConfig(reader io.ReadSeeker) (image.Config, string, error) {
	conf, fmt, err := image.DecodeConfig(reader)
	if err != nil {
		return conf, fmt, err
	}
	reader.Seek(0, io.SeekStart)
	orientation := getOrientation(reader)
	println(orientation)
	switch orientation {
	case "5":
		fallthrough
	case "6":
		fallthrough
	case "7":
		fallthrough
	case "8":
		conf.Width, conf.Height = conf.Height, conf.Width
	}
	return conf, fmt, err
}

func getOrientation(reader io.ReadSeeker) string {
	x, err := exif.Decode(reader)
	if err != nil {
		return "1"
	}
	if x != nil {
		orient, err := x.Get(exif.Orientation)
		if err != nil {
			return "1"
		}
		if orient != nil {
			return orient.String()
		}
	}

	return "1"
}
