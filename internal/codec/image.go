//go:build !libjpeg
// +build !libjpeg

package codec

import (
	"image"
	"image/jpeg"
	"io"
)

func DecodeJpeg(reader io.ReadSeeker) (image.Image, error) {
	return jpeg.Decode(reader)
}

func EncodeJpeg(w io.Writer, image image.Image, quality int) error {
	return jpeg.Encode(w, image, &jpeg.Options{
		Quality: quality,
	})
}
