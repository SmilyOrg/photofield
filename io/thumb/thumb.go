package thumb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	goio "io"

	"image/jpeg"
	"image/png"

	"photofield/io"
	"photofield/io/goimage"
)

// type Fit uint8
type Template = *template.Template

// const (
// 	FitOutside Fit = iota + 1
// 	FitInside
// 	OriginalSize
// )

// var (
// 	FitValue = map[string]uint8{
// 		"INSIDE":   uint8(FitInside),
// 		"OUTSIDE":  uint8(FitOutside),
// 		"ORIGINAL": uint8(OriginalSize),
// 	}
// )

type TemplateData struct {
	Dir      string
	Filename string
}

// func (f *ThumbFit) UnmarshalJSON(data []byte) error {
// 	var s string

//     if err = json.Unmarshal(data, &s); err != nil {
//         return err
//     }

// 	println(s)

// 	// return json.Unmarshal(data, &)
// }

type Thumb struct {
	ThumbName string `json:"name"`

	PathTemplate Template

	Fit io.AspectRatioFit `json:"fit"`

	Width  int `json:"width"`
	Height int `json:"height"`

	goimage goimage.Image
}

func New(
	name string,
	pathTemplate string,
	fit io.AspectRatioFit,
	Width int,
	Height int,
) *Thumb {
	t := &Thumb{
		ThumbName:    name,
		PathTemplate: template.Must(template.New("").Parse(pathTemplate)),
		Fit:          fit,
		Width:        Width,
		Height:       Height,
	}

	// Optimized jpeg/png case, case insensitive
	ext := strings.ToLower(filepath.Ext(pathTemplate))

	switch ext {
	case ".jpg", ".jpeg":
		t.goimage.Decoder = jpeg.Decode

	case ".png":
		t.goimage.Decoder = png.Decode
	}

	return t
}

func (t *Thumb) Close() error {
	return nil
}

func (t Thumb) Name() string {
	return fmt.Sprintf("thumb-%dx%d-%s", t.Width, t.Height, t.ThumbName)
}

func (t Thumb) DisplayName() string {
	return "Pregenerated thumbnail"
}

func (t Thumb) Ext() string {
	return filepath.Ext(t.resolvePath(""))
}

func (t Thumb) Rotate() bool {
	return true
}

func (t Thumb) Size(size io.Size) io.Size {
	return io.Size{X: t.Width, Y: t.Height}.Fit(size, t.Fit)
}

func (t Thumb) GetDurationEstimate(size io.Size) time.Duration {
	return 31 * time.Nanosecond * time.Duration(t.Width*t.Height)
}

func (t *Thumb) resolvePath(originalPath string) string {
	if t.PathTemplate == nil {
		return ""
	}
	var rendered bytes.Buffer
	dir, filename := filepath.Split(originalPath)
	err := t.PathTemplate.Execute(&rendered, TemplateData{
		Dir:      dir,
		Filename: filename,
	})
	if err != nil {
		panic(err)
	}
	return rendered.String()
}

func (t Thumb) Exists(ctx context.Context, id io.ImageId, path string) bool {
	_, err := os.Stat(t.resolvePath(path))
	return !errors.Is(err, os.ErrNotExist)
}

func (t Thumb) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	r := t.goimage.Get(ctx, id, t.resolvePath(path))
	r.Orientation = io.Normal
	return r
}

func (t Thumb) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	t.goimage.Reader(ctx, id, t.resolvePath(path), fn)
}

func (t Thumb) Decode(ctx context.Context, r goio.Reader) io.Result {
	return t.goimage.Decode(ctx, r)
}

func (t Thumb) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}
