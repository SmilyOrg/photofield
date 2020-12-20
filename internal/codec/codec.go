package photofield

import (
	"image"
	"io"
	"time"

	. "photofield/internal"

	"github.com/disintegration/imaging"
	"github.com/rwcarlsen/goexif/exif"
)

type MediaCoder struct {
	infoCoder MediaInfoCoder
}

type MediaInfoCoder interface {
	DecodeInfo(path string, info *ImageInfo) error
	Close()
}

func NewMediaCoder(exifToolCount int) *MediaCoder {
	coder := MediaCoder{}
	// coder.infocoder = NewGoExifRwcarlsencoder()
	// coder.infocoder = NewExifToolBarashercoder(exifToolCount)
	coder.infoCoder = NewExifToolMostlyGeekDecoder(exifToolCount)
	return &coder
}

func (coder *MediaCoder) Close() {
	coder.infoCoder.Close()
}

func (coder *MediaCoder) decodePostProcess(reader io.ReadSeeker, img image.Image) (image.Image, error) {
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

func (coder *MediaCoder) DecodeJpeg(reader io.ReadSeeker) (image.Image, error) {
	img, err := DecodeJpeg(reader)
	if err != nil {
		return img, err
	}
	return coder.decodePostProcess(reader, img)
}

func (coder *MediaCoder) EncodeJpeg(w io.Writer, image image.Image) error {
	return EncodeJpeg(w, image)
}

func (coder *MediaCoder) Decode(reader io.ReadSeeker) (image.Image, string, error) {
	img, fmt, err := image.Decode(reader)
	if err != nil {
		return img, fmt, err
	}
	img, err = coder.decodePostProcess(reader, img)
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

func (coder *MediaCoder) DecodeInfo(path string, info *ImageInfo) error {
	err := coder.infoCoder.DecodeInfo(path, info)
	// println(path, info.Width, info.Height, info.DateTime.String())
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

func (coder *MediaCoder) DecodeConfig(reader io.ReadSeeker) (image.Config, string, error) {
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
