package image

import (
	"image"
	"io"
	"os"
	"photofield/tag"

	"github.com/rwcarlsen/goexif/exif"
)

type GoExifRwcarlsenLoader struct{}

func NewGoExifRwcarlsenLoader() *GoExifRwcarlsenLoader {
	return &GoExifRwcarlsenLoader{}
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

func (decoder *GoExifRwcarlsenLoader) DecodeInfo(path string, info *Info) ([]tag.Tag, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return nil, decoder.DecodeInfoReader(file, info)
}

func (decoder *GoExifRwcarlsenLoader) DecodeInfoReader(r io.ReadSeeker, info *Info) error {
	x, err := exif.Decode(r)
	if err == nil {
		info.DateTime, _ = x.DateTime()
	}

	orientation := parseOrientation(getOrientationFromExif(x))

	r.Seek(0, io.SeekStart)
	conf, _, err := image.DecodeConfig(r)
	if err != nil {
		return err
	}

	if orientation.SwapsDimensions() {
		conf.Width, conf.Height = conf.Height, conf.Width
	}

	info.Width, info.Height = conf.Width, conf.Height
	info.Orientation = orientation

	return nil
}

func (decoder *GoExifRwcarlsenLoader) DecodeBytes(path string, tagName string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return nil, err
	}

	tag, err := x.Get(exif.FieldName(tagName))
	if err != nil {
		return nil, err
	}

	return tag.Val, nil
}

func (decoder *GoExifRwcarlsenLoader) Close() {}
