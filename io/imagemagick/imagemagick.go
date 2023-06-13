package imagemagick

import (
	"context"
	"fmt"
	"image"
	"log"
	"os/exec"
	"photofield/io"
	"strconv"
	"time"
)

var (
	ErrMissingBinary = fmt.Errorf("imagemagick binary not found")
)

// ImageMagick is a wrapper for the ImageMagick command line tool.
type ImageMagick struct {
	// Path to the ImageMagick executable.
	Path   string
	Width  int
	Height int
	Fit    io.AspectRatioFit
}

// FindPath finds the ImageMagick executable.
func FindPath() string {
	path, err := exec.LookPath("magick")
	if err != nil {
		log.Printf("imagemagick not found: %s\n", err.Error())
		return ""
	}
	return path
}

func (i ImageMagick) Name() string {
	var fit string
	switch i.Fit {
	case io.FitInside:
		fit = "in"
	case io.FitOutside:
		fit = "out"
	case io.OriginalSize:
		fit = "orig"
	}
	found := ""
	if i.Path == "" {
		found = " (N/A)"
	}
	return fmt.Sprintf("imagemagick-%dx%d-%s%s", i.Width, i.Height, fit, found)
}

func (i ImageMagick) DisplayName() string {
	return "ImageMagick JPEG"
}

func (i ImageMagick) Ext() string {
	return ".jpg"
}

func (i ImageMagick) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return true
}

func (i ImageMagick) Size(size io.Size) io.Size {
	return io.Size{X: i.Width, Y: i.Height}.Fit(size, io.FitInside)
}

func (i ImageMagick) GetDurationEstimate(size io.Size) time.Duration {
	return 30 * time.Nanosecond * time.Duration(size.Area())
}

func (i ImageMagick) Rotate() bool {
	return false
}

func (i ImageMagick) ForceOriginalAspectRatio() string {
	return "decrease"
}

func (i ImageMagick) thumbnailSize(id io.ImageId) string {
	if i.Width == 0 || i.Height == 0 {
		return ""
	}
	return strconv.Itoa(i.Width) + "x" + strconv.Itoa(i.Height)
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

func (i ImageMagick) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	if i.Path == "" {
		return io.Result{Error: ErrMissingBinary}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		i.Path,
		"-quiet",
		path,
		"-thumbnail", i.thumbnailSize(id),
		"-gravity", "center",
		"-depth", "8",
		"-alpha", "on",
		"pam:-",
	)

	// println(cmd.String())
	b, err := cmd.Output()
	err = formatErr(err, "imagemagick")
	if err != nil {
		return io.Result{Error: err}
	}

	pam, err := readPAM(b)
	if err != nil {
		return io.Result{Error: err}
	}

	if pam.Depth != 4 {
		return io.Result{Error: fmt.Errorf("unexpected depth %d", pam.Depth)}
	}

	if pam.MaxValue != 255 {
		return io.Result{Error: fmt.Errorf("unexpected max value %d", pam.MaxValue)}
	}

	if pam.Width < 0 || pam.Height < 0 {
		return io.Result{Error: fmt.Errorf("unexpected size %d x %d", pam.Width, pam.Height)}
	}

	log.Printf("ImageMagick: %v %dx%d\n", id, pam.Width, pam.Height)

	rgba := image.RGBA{
		Pix:    pam.Bytes,
		Stride: 4 * pam.Width,
		Rect:   image.Rect(0, 0, pam.Width, pam.Height),
	}
	return io.Result{Image: image.Image(&rgba)}
}
