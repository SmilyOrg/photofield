package webp

import (
	"image"
	"io"
	dynamic "photofield/internal/codec/webp/jack/dynamic"
	transpiled "photofield/internal/codec/webp/jack/transpiled"
)

// The original library has a bug where quality is clamped to 0-1, but it should
// be 0-100. Therefore, we reimplement the thin wrapper here to fix that.

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

	// Convert to NRGBA if needed
	nrgbaImg, ok := img.(*image.NRGBA)
	if !ok {
		// Convert to NRGBA
		bounds := img.Bounds()
		nrgbaImg = image.NewNRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				nrgbaImg.Set(x, y, img.At(x, y))
			}
		}
	}

	err := dynamic.Encode(writer, nrgbaImg, quality)
	if err == dynamic.ErrNotSupported {
		return transpiled.Encode(writer, nrgbaImg, quality)
	}
	return err
}
