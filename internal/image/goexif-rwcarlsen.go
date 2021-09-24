package image

import (
	"image"
	"io"
	"os"

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

func (decoder *GoExifRwcarlsenLoader) DecodeInfo(path string, info *Info) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err == nil {
		info.DateTime, _ = x.DateTime()
	}

	orientation := parseOrientation(getOrientationFromExif(x))

	file.Seek(0, io.SeekStart)
	conf, _, err := image.DecodeConfig(file)
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

func (decoder *GoExifRwcarlsenLoader) Close() {}
