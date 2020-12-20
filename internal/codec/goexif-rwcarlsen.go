package photofield

import (
	"image"
	"io"
	"os"
	. "photofield/internal"

	"github.com/rwcarlsen/goexif/exif"
)

type GoExifRwcarlsenDecoder struct{}

func NewGoExifRwcarlsenDecoder() *GoExifRwcarlsenDecoder {
	return nil
}

func getPortraitFromExif(x *exif.Exif) bool {
	return getPortraitFromOrientation(getOrientationFromExif(x))
}

func (decoder *GoExifRwcarlsenDecoder) DecodeInfo(path string, info *ImageInfo) error {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return err
	}

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

func (decoder *GoExifRwcarlsenDecoder) Close() {}
