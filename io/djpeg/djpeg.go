package djpeg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os/exec"
	"photofield/io"
	"runtime/trace"
	"time"

	goio "io"

	pnm "github.com/jbuchbinder/gopnm"
	"golang.org/x/image/draw"
)

var (
	ErrMissingBinary = fmt.Errorf("djpeg binary not found")
)

type Djpeg struct {
	Path   string
	Width  int
	Height int
	Scale  string
	ScaleM int
	ScaleN int
}

func FindPath() string {
	path, err := exec.LookPath("djpeg")
	if err != nil {
		log.Printf("djpeg not found: %s\n", err.Error())
		return ""
	}
	log.Printf("djpeg found at %s\n", path)
	return path
}

func (o Djpeg) Close() error {
	return nil
}

func (o Djpeg) Name() string {
	found := ""
	if o.Path == "" {
		found = " (N/A)"
	}
	if o.Resized() {
		return fmt.Sprintf("djpeg-%dx%d%s", o.Width, o.Height, found)
	}
	return fmt.Sprintf("djpeg%d%d%s", o.ScaleM, o.ScaleN, found)
}

func (o Djpeg) DisplayName() string {
	return "djpeg"
}

func (o Djpeg) Ext() string {
	return ".jpg"
}

func (o Djpeg) Resized() bool {
	return o.Width != 0 && o.Height != 0
}

func (o Djpeg) Size(size io.Size) io.Size {
	if o.Resized() {
		return io.Size{
			X: o.Width,
			Y: o.Height,
		}
	}
	if o.ScaleM != 0 && o.ScaleN != 0 {
		return io.Size{
			X: size.X * o.ScaleM / o.ScaleN,
			Y: size.Y * o.ScaleM / o.ScaleN,
		}
	}
	return size
}

func (o Djpeg) GetDurationEstimate(size io.Size) time.Duration {
	return 30 * time.Nanosecond * time.Duration(size.Area())
}

func (o Djpeg) Rotate() bool {
	return false
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

func (o Djpeg) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return true
}

func (o Djpeg) Get(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	defer trace.StartRegion(ctx, "djpeg.Get").End()

	if o.Path == "" {
		return io.Result{Error: ErrMissingBinary}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		o.Path,
		"-pnm",
		"-scale", o.Scale,
		path,
	)

	trace.Log(ctx, "cmd", cmd.String())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return io.Result{Error: err}
	}
	if err := cmd.Start(); err != nil {
		return io.Result{Error: formatErr(err, "djpeg")}
	}

	pnmd := trace.StartRegion(ctx, "pnm decode")
	img, err := pnm.Decode(stdout)
	pnmd.End()
	if err != nil {
		return io.Result{Error: err}
	}

	if err := cmd.Wait(); err != nil {
		return io.Result{Error: formatErr(err, "djpeg")}
	}

	if o.Resized() {
		img = resize(img, o.Width, o.Height)
	}

	return io.Result{
		Image:       img,
		Error:       err,
		Orientation: io.SourceInfoOrientation,
	}
}

func (o Djpeg) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	r := o.Get(ctx, id, path, io.Size{})
	if r.Error != nil {
		fn(nil, r.Error)
		return
	}
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, r.Image, nil)
	if err != nil {
		fn(nil, err)
		return
	}
	rd := bytes.NewReader(buf.Bytes())
	fn(rd, nil)
}

func (o Djpeg) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}

func formatErr(err error, cmdName string) error {
	if exiterr, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf(
			"%s error (exit code %d)\n%s",
			cmdName, exiterr.ExitCode(), exiterr.Stderr,
		)
	}
	return err
}
