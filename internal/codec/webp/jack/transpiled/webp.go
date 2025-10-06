//go:build !race && ((linux && (amd64 || arm64)) || (darwin && (amd64 || arm64)) || (windows && (amd64 || arm64)))

package webp

import (
	"errors"
	"image"
	"io"

	"git.sr.ht/~jackmordaunt/go-libwebp/lib/common"
	"git.sr.ht/~jackmordaunt/go-libwebp/lib/transpiled/webp"
	"modernc.org/libc"
)

// ErrNotSupported is defined for consistency but not used on supported architectures
var ErrNotSupported = errors.New("webp transpiled encoder not supported on this architecture")

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

	tls := libc.NewTLS()
	defer tls.Close()

	return common.Encode(writer, nrgbaImg, float32(quality),
		func(in uintptr, w, h, bps int32, q float32, out uintptr) uint64 {
			return webp.WebPEncodeRGBA(tls, in, w, h, bps, q, out)
		},
		func(p uintptr) {
			webp.WebPFree(tls, p)
		},
	)
}
