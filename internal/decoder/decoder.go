package decoder

import (
	"image"
	"io"
	"time"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

type Info struct {
	Config   *image.Config
	DateTime time.Time
}

func Decode(reader io.ReadSeeker) (image.Image, string, error) {
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

func DecodeInfo(reader io.ReadSeeker) (Info, error) {
	info := Info{}

	x, err := exif.Decode(reader)
	if err == nil {
		info.DateTime, _ = x.DateTime()
	}

	portrait := getPortraitFromExif(x)
	reader.Seek(0, io.SeekStart)
	conf, _, err := image.DecodeConfig(reader)
	if err != nil {
		return info, err
	}
	if portrait {
		conf.Width, conf.Height = conf.Height, conf.Width
	}

	if err != nil {
		return info, err
	}
	info.Config = &conf

	return info, nil
}

func getPortraitFromExif(x *exif.Exif) bool {
	orientation := getOrientationFromExif(x)
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

func DecodeConfig(reader io.ReadSeeker) (image.Config, string, error) {
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
