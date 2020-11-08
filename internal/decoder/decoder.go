package photofield

import (
	"image"
	"io"
	"time"

	. "photofield/internal"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

type MediaDecoder struct {
	infoDecoder ImageInfoDecoder
}

type ImageInfoDecoder interface {
	DecodeInfo(path string, info *ImageInfo) error
	Close()
}

func NewMediaDecoder(exifToolCount int) *MediaDecoder {
	decoder := MediaDecoder{}
	decoder.infoDecoder = NewGoExifRwcarlsenDecoder()
	// decoder.infoDecoder = NewExifToolBarasherDecoder(exifToolCount)
	// decoder.infoDecoder = NewExifToolMostlyGeekDecoder(exifToolCount)
	return &decoder
}

func (decoder *MediaDecoder) Close() {
	decoder.infoDecoder.Close()
}

func (decoder *MediaDecoder) Decode(reader io.ReadSeeker) (image.Image, string, error) {
	img, fmt, err := image.Decode(reader)
	if err != nil {
		return img, fmt, err
	}
	reader.Seek(0, io.SeekStart)
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

	return img, fmt, err
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

func (decoder *MediaDecoder) DecodeInfo(path string, info *ImageInfo) error {
	err := decoder.infoDecoder.DecodeInfo(path, info)
	println(path, info.Width, info.Height, info.DateTime.String())
	return err
}

func getPortraitFromRotation(rotation string) bool {
	switch rotation {
	case "90":
		fallthrough
	case "270":
		return true
	default:
		return false
	}
}

func getPortraitFromOrientation(orientation string) bool {
	switch orientation {
	case "1":
		fallthrough
	case "2":
		fallthrough
	case "3":
		fallthrough
	case "4":
		return false
	case "5":
		fallthrough
	case "6":
		fallthrough
	case "7":
		fallthrough
	case "8":
		return true
	default:
		return false
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

func (decoder *MediaDecoder) DecodeConfig(reader io.ReadSeeker) (image.Config, string, error) {
	conf, fmt, err := image.DecodeConfig(reader)
	if err != nil {
		return conf, fmt, err
	}
	reader.Seek(0, io.SeekStart)
	orientation := getOrientation(reader)
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
