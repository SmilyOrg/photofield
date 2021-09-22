package image

import (
	"image"
	"io"
	"os"

	"github.com/rwcarlsen/goexif/exif"
)

type GoExifRwcarlsenLoader struct{}

func NewGoExifRwcarlsenLoader() *GoExifRwcarlsenLoader {
	return nil
}

func getPortraitFromExif(x *exif.Exif) bool {
	return getOrientationDimensionSwap(getOrientationFromExif(x))
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

	portrait := getPortraitFromExif(x)
	file.Seek(0, io.SeekStart)
	conf, _, err := image.DecodeConfig(file)
	if err != nil {
		return err
	}
	if portrait {
		conf.Width, conf.Height = conf.Height, conf.Width
	}

	if err != nil {
		return err
	}
	info.Width, info.Height = conf.Width, conf.Height

	return nil
}

func (decoder *GoExifRwcarlsenLoader) Close() {}
