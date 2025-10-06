package webp

import (
	"image"
	"io"

	"github.com/HugoSmits86/nativewebp"
)

// Encode writes the image to the writer as WebP with the specified quality
// quality parameter is currently ignored by the native implementation
func Encode(writer io.Writer, img image.Image, quality int) error {
	return nativewebp.Encode(writer, img, &nativewebp.Options{})
}
