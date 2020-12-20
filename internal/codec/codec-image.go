// +build !libjpeg

package photofield

import (
	"image"
	"image/jpeg"
	"io"
)

func DecodeJpeg(reader io.ReadSeeker) (image.Image, error) {
	return jpeg.Decode(reader)
}

func EncodeJpeg(w io.Writer, image image.Image) error {
	return jpeg.Encode(w, image, &jpeg.Options{
		Quality: 80,
	})
}
