//go:build race

package webp

import (
	"errors"
	"image"
	"io"
)

// Why is this disabled under -race?
//
// The transpiled encoder (go-libwebp "transpiled" backend) relies on C-style
// pointer arithmetic over a single RGBA pixel buffer. The original C code does
// things like:
//   alpha = rgba + 3; alpha[i*4]
// which, when mechanically translated to Go, becomes uintptr arithmetic that
// produces addresses (alpha + i*4) that can move past the end of the original
// allocation for small images (e.g. 1x1). Even if those pointers are never
// dereferenced out-of-bounds, Go's checkptr instrumentation (implicitly
// enabled when the race detector is on) treats creation of such pointers as a
// violation and aborts the program with a fatal runtime error.
//
// In short:
//   * C permits this pattern; Go's memory safety model does not.
//   * Without -race: the code "works" (undefined behavior that happens not to crash).
//   * With -race (and therefore checkptr): runtime.checkptrArithmetic triggers.
//
// Disabling the transpiled path under -race keeps the rest of the application
// testable with the race detector while still allowing the dynamic
// backend or other encoders (jpeg/png) to function. The dynamic WebP encoder
// (jackdyn) calls into real libwebp and is not subject to checkptr in
// the same way, so it remains available.

// ErrNotSupported is returned when attempting to use the transpiled webp encoder under the race detector.
var ErrNotSupported = errors.New("webp transpiled encoder disabled under race detector")

// Encode always returns ErrNotSupported when built with -race.
func Encode(w io.Writer, img image.Image, quality int) error { return ErrNotSupported }
