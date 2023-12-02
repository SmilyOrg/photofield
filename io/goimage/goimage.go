package goimage

import (
	"context"
	"image"
	"os"
	"photofield/io"
	"time"

	goio "io"

	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

type Image struct {
	Width   int
	Height  int
	Decoder func(goio.Reader) (image.Image, error)
}

func (o Image) Close() error {
	return nil
}

func (o Image) Name() string {
	return "original"
}

func (o Image) DisplayName() string {
	return "Original"
}

func (o Image) Ext() string {
	return ""
}

func (o Image) Resized() bool {
	return o.Width != 0 && o.Height != 0
}

func (o Image) Size(size io.Size) io.Size {
	if o.Resized() {
		return io.Size{
			X: o.Width,
			Y: o.Height,
		}
	}
	return size
}

func (o Image) GetDurationEstimate(size io.Size) time.Duration {
	return 30 * time.Nanosecond * time.Duration(size.Area())
}

func (o Image) Rotate() bool {
	return true
}

func resize(img image.Image, maxWidth, maxHeight int) image.Image {
	origW := img.Bounds().Size().X
	origH := img.Bounds().Size().Y
	aspectRatio := float64(origW) / float64(origH)

	desiredW := maxWidth
	desiredH := maxHeight
	if float64(desiredW)/float64(desiredH) > aspectRatio {
		desiredW = int(float64(desiredH) * aspectRatio)
	} else {
		desiredH = int(float64(desiredW) / aspectRatio)
	}
	resized := image.NewRGBA(image.Rect(0, 0, desiredW, desiredH))
	draw.ApproxBiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Src, nil)
	return resized
}

func (o Image) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return true
}

func (o Image) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	f, err := os.Open(path)
	if err != nil {
		return io.Result{Error: err}
	}
	defer f.Close()

	var img image.Image
	if o.Decoder != nil {
		img, err = o.Decoder(f)
	} else {
		img, _, err = image.Decode(f)
	}

	if o.Resized() && err == nil {
		img = resize(img, o.Width, o.Height)
	}

	return io.Result{
		Image:       img,
		Error:       err,
		Orientation: io.SourceInfoOrientation,
	}
}

func (o Image) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	f, err := os.Open(path)
	if err != nil {
		fn(nil, err)
		return
	}
	defer f.Close()

	fn(f, nil)
}

func (o Image) Decode(ctx context.Context, r goio.Reader) io.Result {
	img, _, err := image.Decode(r)
	if o.Resized() && err == nil {
		img = resize(img, o.Width, o.Height)
	}
	return io.Result{
		Image:       img,
		Error:       err,
		Orientation: io.SourceInfoOrientation,
	}
}

func (o Image) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}
