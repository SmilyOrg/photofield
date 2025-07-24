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

	"github.com/spakin/netpbm"
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

	// Run djpeg to check if it's libjpeg-turbo
	cmd := exec.Command(path, "-version")
	output, err := cmd.CombinedOutput()
	if err == nil {
		if !bytes.Contains(output, []byte("libjpeg-turbo")) {
			log.Println("djpeg warning: version is not libjpeg-turbo, performance may be suboptimal")
		}
	}
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

	// Don't resize if already smaller than max dimensions
	if origW <= maxWidth && origH <= maxHeight {
		return img
	}

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

// getMinimalScale finds the smallest djpeg scale that produces an image
// larger than the target dimensions in both width and height
func getMinimalScale(origWidth, origHeight, targetWidth, targetHeight int) string {
	if origWidth <= targetWidth && origHeight <= targetHeight {
		return "8/8" // No scaling needed
	}
	for i := 1; i <= 8; i++ {
		scaledWidth := origWidth * i / 8
		scaledHeight := origHeight * i / 8
		if scaledWidth >= targetWidth && scaledHeight >= targetHeight {
			return fmt.Sprintf("%d/8", i)
		}
	}
	return "8/8"
}

// run runs djpeg with the specified scale and returns the decoded image
func (o Djpeg) run(ctx context.Context, path, scale string) (image.Image, error) {
	cmd := exec.CommandContext(
		ctx,
		o.Path,
		"-pnm",
		"-scale", scale,
		path,
	)

	trace.Log(ctx, "cmd", cmd.String())

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, formatErr(err, "djpeg")
	}

	pnmd := trace.StartRegion(ctx, "pnm decode")
	img, err := netpbm.Decode(stdout, nil)
	pnmd.End()
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return nil, formatErr(err, "djpeg")
	}

	return img, nil
}

func (o Djpeg) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	defer trace.StartRegion(ctx, "djpeg.Get").End()

	if o.Path == "" {
		return io.Result{Error: ErrMissingBinary}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	scale := o.Scale
	var img image.Image
	var err error

	// If Width/Height are specified but Scale is empty, use two-pass approach
	if o.Resized() && scale == "" {
		// First pass: try with 1/8 scale to get dimensions quickly
		imgSmall, err := o.run(ctx, path, "1/8")
		if err != nil {
			return io.Result{Error: err}
		}

		// Check if 1/8 scale is sufficient for our target size
		if imgSmall.Bounds().Dx() >= o.Width && imgSmall.Bounds().Dy() >= o.Height {
			img = imgSmall
		} else {
			// Second pass: calculate optimal scale and decode again

			// Calculate original dimensions from 1/8 scale image
			origWidth := imgSmall.Bounds().Dx() * 8
			origHeight := imgSmall.Bounds().Dy() * 8
			optimalScale := getMinimalScale(origWidth, origHeight, o.Width, o.Height)
			img, err = o.run(ctx, path, optimalScale)
			if err != nil {
				return io.Result{Error: err}
			}
		}
	} else {
		// Single pass: use specified scale or default
		if scale == "" {
			scale = "8/8"
		}
		img, err = o.run(ctx, path, scale)
		if err != nil {
			return io.Result{Error: err}
		}
	}

	if o.Resized() {
		img = resize(img, o.Width, o.Height)
	}

	return io.Result{
		Image:       img,
		Error:       nil,
		Orientation: io.SourceInfoOrientation,
	}
}

func (o Djpeg) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	r := o.Get(ctx, id, path)
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

func (o Djpeg) GetWithSize(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	defer trace.StartRegion(ctx, "djpeg.GetWithSize").End()

	if o.Path == "" {
		return io.Result{Error: ErrMissingBinary}
	}

	// If either dimension is 0, fall back to Get
	if original.X == 0 || original.Y == 0 {
		return o.Get(ctx, id, path)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	scale := o.Scale
	if o.Resized() && scale == "" {
		scale = getMinimalScale(original.X, original.Y, o.Width, o.Height)
	}

	img, err := o.run(ctx, path, scale)
	if err != nil {
		return io.Result{Error: err}
	}

	if o.Resized() {
		img = resize(img, o.Width, o.Height)
	}

	return io.Result{
		Image:       img,
		Error:       nil,
		Orientation: io.SourceInfoOrientation,
	}
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
