package avif

import (
	"image"
	"io"

	"github.com/gen2brain/avif"
)

// Encode writes the image to the writer as AVIF with the specified quality
// quality should be between 0-100, with higher values meaning better quality
func Encode(writer io.Writer, img image.Image, quality int) error {
	// Ensure quality is within valid range
	if quality < 0 {
		quality = 0
	}
	if quality > 100 {
		quality = 100
	}

	opts := avif.Options{}
	if quality != 0 {
		opts.Quality = quality
	}
	return avif.Encode(writer, img, opts)
}
