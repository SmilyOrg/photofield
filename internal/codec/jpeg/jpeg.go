package jpeg

import (
	"image"
	"image/jpeg"
	"io"
)

// Encode writes the image to the writer as JPEG with the specified quality
// quality should be between 1-100, with higher values meaning better quality
func Encode(writer io.Writer, img image.Image, quality int) error {
	// Ensure quality is within valid range
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	options := &jpeg.Options{Quality: quality}
	return jpeg.Encode(writer, img, options)
}
