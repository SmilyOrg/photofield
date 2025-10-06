package webp

import (
	"image"
	"io"

	"github.com/chai2010/webp"
)

// Encode writes the image to the writer as WebP with the specified quality
// quality should be between 0-100, with higher values meaning better quality
func Encode(writer io.Writer, img image.Image, quality int) error {
	// Ensure quality is within valid range
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}

	opts := &webp.Options{}
	if quality != 0 {
		opts.Quality = float32(quality)
	}
	return webp.Encode(writer, img, opts)
}
