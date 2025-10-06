//go:build !(linux && (amd64 || arm64)) && !(darwin && (amd64 || arm64)) && !(windows && (amd64 || arm64))

package webp

import (
	"errors"
	"image"
	"io"
)

var ErrNotSupported = errors.New("webp-jackdyn not supported on this architecture")

// Encode returns an error on unsupported architectures
func Encode(writer io.Writer, img image.Image, quality int) error {
	return ErrNotSupported
}
