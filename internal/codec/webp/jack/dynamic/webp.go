package webp

import (
	"errors"
	"image"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/lib/common"
	"git.sr.ht/~jackmordaunt/go-libwebp/lib/dynamic/webp"
)

var (
	supported       = false
	ErrNotSupported = errors.New("webp-dynamic not supported: library not found")
)

func init() {
	err := webp.Init()
	if err != nil {
		return
	}
	supported = true
}

// Encode writes the image to the writer as WebP with the specified quality
// quality should be between 0-100, with higher values meaning better quality
func Encode(writer io.Writer, img image.Image, quality int) error {
	if !supported {
		return ErrNotSupported
	}

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

	return common.Encode(writer, nrgbaImg, float32(quality), webp.WebPEncodeRGBA, webp.WebPFree)
}
