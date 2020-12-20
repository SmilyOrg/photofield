// +build libjpeg

package photofield

import (
	"image"
	"io"

	"github.com/pixiv/go-libjpeg/jpeg"
)

func DecodeJpeg(reader io.ReadSeeker) (image.Image, error) {
	return jpeg.Decode(reader, &jpeg.DecoderOptions{})
}

func EncodeJpeg(w io.Writer, image image.Image) error {
	return jpeg.Encode(w, image, &jpeg.EncoderOptions{
		Quality: 80,
	})
}
