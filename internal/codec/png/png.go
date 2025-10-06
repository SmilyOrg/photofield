package png

import (
	"image"
	"image/png"
	"io"
)

// Encode writes the image to the writer as PNG
// quality parameter is ignored for PNG as it's a lossless format
func Encode(writer io.Writer, img image.Image, quality int) error {
	return png.Encode(writer, img)
}
