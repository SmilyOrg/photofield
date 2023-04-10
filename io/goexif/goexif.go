package goexif

import (
	"bytes"
	"context"
	"os"
	"photofield/io"
	"strconv"
	"time"

	goio "io"

	"image/jpeg"

	"github.com/rwcarlsen/goexif/exif"
)

type Exif struct {
	// Tag string `json:"tag"`
	Width  int `json:"width"`
	Height int `json:"height"`
	Fit    io.AspectRatioFit
}

func (e Exif) Name() string {
	return "goexif"
}

func (e Exif) DisplayName() string {
	return "EXIF Thumbnail"
}

func (e Exif) Ext() string {
	return ".jpg"
}

func (e Exif) Size(size io.Size) io.Size {
	return io.Size{X: e.Width, Y: e.Height}.Fit(size, e.Fit)
}

func (e Exif) GetDurationEstimate(size io.Size) time.Duration {
	// return 862 * time.Microsecond // SSD
	// return 10 * time.Millisecond // SSD - real world
	return 100 * time.Millisecond // penalized
	// return 930 * time.Microsecond // HDD
}

func (e Exif) Rotate() bool {
	return false
}

func (e Exif) Exists(ctx context.Context, id io.ImageId, path string) bool {
	exists := false
	e.Reader(ctx, id, path, func(r goio.ReadSeeker, err error) {
		if r != nil || err == nil {
			exists = true
		}
	})
	return exists
}

func (e Exif) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	b, o, err := load(path)
	if err != nil {
		return io.Result{Error: err}
	}

	r := bytes.NewReader(b)
	img, err := jpeg.Decode(r)
	return io.Result{
		Image:       img,
		Orientation: o,
		Error:       err,
	}
}

func (e Exif) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	b, _, err := load(path)
	if err != nil {
		fn(nil, err)
		return
	}

	fn(bytes.NewReader(b), nil)
}

func (e Exif) Decode(ctx context.Context, r goio.Reader) io.Result {
	img, err := jpeg.Decode(r)
	return io.Result{
		Image:       img,
		Orientation: io.Normal,
		Error:       err,
	}
}

func (e Exif) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}

func load(path string) ([]byte, io.Orientation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, io.Normal, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return nil, io.Normal, err
	}

	b, err := x.JpegThumbnail()
	o := getOrientation(x)
	return b, o, err
}

func getOrientation(x *exif.Exif) io.Orientation {
	i, err := getTagInt(x, exif.Orientation)
	if err != nil {
		return io.Normal
	}
	return io.Orientation(i)
	// s, err := getTagString(x, exif.Orientation)
	// if err != nil {
	// 	println(err.Error())
	// 	return io.Normal
	// }
	// return parseOrientation(s)
}

func getTagString(x *exif.Exif, name exif.FieldName) (string, error) {
	t, err := x.Get(exif.Orientation)
	if err != nil {
		return "", err
	}

	s, err := t.StringVal()
	if err != nil {
		return "", err
	}

	return s, nil
}

func getTagInt(x *exif.Exif, name exif.FieldName) (int, error) {
	t, err := x.Get(exif.Orientation)
	if err != nil {
		return 0, err
	}
	i, err := t.Int(0)
	if err != nil {
		return i, err
	}
	return i, nil
}

func parseOrientation(orientation string) io.Orientation {
	n, err := strconv.Atoi(orientation)
	if err != nil || n < 1 || n > 8 {
		return io.Normal
	}
	return io.Orientation(n)
}

// func getOrientationFromRotation(rotation string) io.Orientation {
// 	switch rotation {
// 	case "0":
// 		return Normal
// 	case "90":
// 		return Rotate90
// 	case "180":
// 		return Rotate180
// 	case "270":
// 		return Rotate270
// 	default:
// 		return Normal
// 	}
// }
