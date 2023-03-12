package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"log"
	"os/exec"
	"photofield/io"
	"strconv"
	"time"

	goio "io"

	"image/jpeg"
)

// type ForceOriginalAspectRatio string

// const (
// 	Decrease ForceOriginalAspectRatio = "decrease"
// 	Increase ForceOriginalAspectRatio = "increase"
// )

// type Fit uint8

// const (
// 	FitOutside Fit = iota + 1
// 	FitInside
// )

var (
	ErrMissingBinary = fmt.Errorf("ffmpeg binary not found")
)

type FFmpeg struct {
	Path   string
	Width  int
	Height int
	Fit    io.AspectRatioFit
}

func FindPath() string {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Printf("ffmpeg not found: %s\n", err.Error())
		return ""
	}
	log.Printf("ffmpeg found at %s\n", path)
	return path
}

func (f FFmpeg) Name() string {
	var fit string
	switch f.Fit {
	case io.FitInside:
		fit = "in"
	case io.FitOutside:
		fit = "out"
	case io.OriginalSize:
		fit = "orig"
	}
	found := ""
	if f.Path == "" {
		found = " (N/A)"
	}
	return fmt.Sprintf("ffmpeg-%dx%d-%s%s", f.Width, f.Height, fit, found)
}

func (f FFmpeg) Ext() string {
	return ".jpg"
}

func (f FFmpeg) Size(size io.Size) io.Size {
	return io.Size{X: f.Width, Y: f.Height}.Fit(size, f.Fit)
}

func (f FFmpeg) GetDurationEstimate(size io.Size) time.Duration {
	// return 30 * time.Nanosecond * time.Duration(size.Area())
	return 30 * time.Nanosecond * time.Duration(size.Area())
}

func (f FFmpeg) Rotate() bool {
	return false
}

func (f FFmpeg) ForceOriginalAspectRatio() string {
	switch f.Fit {
	case io.FitInside:
		return "decrease"
	case io.FitOutside:
		return "increase"
	}
	return "unknown"
}

func (f FFmpeg) FilterGraph() string {
	if f.Fit == io.OriginalSize {
		return "null"
	}
	foar := f.ForceOriginalAspectRatio()
	return fmt.Sprintf(
		"scale='min(iw,%d)':'min(ih,%d)':force_original_aspect_ratio=%s",
		// "scale_npp='min(iw,%d)':'min(ih,%d)':force_original_aspect_ratio=%s",
		f.Width, f.Height, foar,
	)
}

func (f FFmpeg) exec(ctx context.Context, id io.ImageId, path string) (*exec.Cmd, error) {
	if f.Path == "" {
		return nil, ErrMissingBinary
	}

	cmd := exec.CommandContext(
		ctx,
		f.Path,
		"-hide_banner",
		"-loglevel", "error",
		"-i", path,
		"-vframes", "1",
		"-vf", f.FilterGraph(),
		// "-q:v", "2",
		// "-f", "image2pipe", // jpeg
		"-c:v", "pam",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-an", // no audio
		"-",
	)
	return cmd, nil
}

func (f FFmpeg) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return true
}

func (f FFmpeg) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd, err := f.exec(ctx, id, path)
	if err != nil {
		return io.Result{Error: err}
	}

	// println(cmd.String())
	b, err := cmd.Output()
	err = formatErr(err, "ffmpeg")
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

	rgba := image.RGBA{
		Pix:    pam.Bytes,
		Stride: 4 * pam.Width,
		Rect:   image.Rect(0, 0, pam.Width, pam.Height),
	}
	return io.Result{Image: image.Image(&rgba)}
}

func (f FFmpeg) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd, err := f.exec(ctx, id, path)
	if err != nil {
		fn(nil, err)
		return
	}

	// println(cmd.String())
	b, err := cmd.Output()
	err = formatErr(err, "ffmpeg")
	if err != nil {
		fn(nil, err)
		return
	}

	pam, err := readPAM(b)
	if err != nil {
		fn(nil, err)
		return
	}

	if pam.Depth != 4 {
		fn(nil, fmt.Errorf("unexpected depth %d", pam.Depth))
		return
	}

	if pam.MaxValue != 255 {
		fn(nil, fmt.Errorf("unexpected max value %d", pam.MaxValue))
		return
	}

	if pam.Width < 0 || pam.Height < 0 {
		fn(nil, fmt.Errorf("unexpected size %d x %d", pam.Width, pam.Height))
		return
	}

	rgba := image.RGBA{
		Pix:    pam.Bytes,
		Stride: 4 * pam.Width,
		Rect:   image.Rect(0, 0, pam.Width, pam.Height),
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, &rgba, nil)
	if err != nil {
		fn(nil, err)
		return
	}

	r := bytes.NewReader(buf.Bytes())
	fn(r, nil)
}

func (f FFmpeg) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}

var pam_prefix_magic = []byte("P7\n")
var pam_header_end = []byte("ENDHDR\n")

type metadata struct {
	Streams []struct {
		Index              int    `json:"index"`
		CodecName          string `json:"codec_name"`
		CodecLongName      string `json:"codec_long_name"`
		CodecType          string `json:"codec_type"`
		CodecTagString     string `json:"codec_tag_string"`
		CodecTag           string `json:"codec_tag"`
		Width              int    `json:"width"`
		Height             int    `json:"height"`
		CodedWidth         int    `json:"coded_width"`
		CodedHeight        int    `json:"coded_height"`
		ClosedCaptions     int    `json:"closed_captions"`
		FilmGrain          int    `json:"film_grain"`
		HasBFrames         int    `json:"has_b_frames"`
		SampleAspectRatio  string `json:"sample_aspect_ratio"`
		DisplayAspectRatio string `json:"display_aspect_ratio"`
		PixFmt             string `json:"pix_fmt"`
		Level              int    `json:"level"`
		ColorRange         string `json:"color_range"`
		Refs               int    `json:"refs"`
		RFrameRate         string `json:"r_frame_rate"`
		AvgFrameRate       string `json:"avg_frame_rate"`
		TimeBase           string `json:"time_base"`
		NbReadFrames       string `json:"nb_read_frames"`
		Disposition        struct {
			Default         int `json:"default"`
			Dub             int `json:"dub"`
			Original        int `json:"original"`
			Comment         int `json:"comment"`
			Lyrics          int `json:"lyrics"`
			Karaoke         int `json:"karaoke"`
			Forced          int `json:"forced"`
			HearingImpaired int `json:"hearing_impaired"`
			VisualImpaired  int `json:"visual_impaired"`
			CleanEffects    int `json:"clean_effects"`
			AttachedPic     int `json:"attached_pic"`
			TimedThumbnails int `json:"timed_thumbnails"`
			Captions        int `json:"captions"`
			Descriptions    int `json:"descriptions"`
			Metadata        int `json:"metadata"`
			Dependent       int `json:"dependent"`
			StillImage      int `json:"still_image"`
		} `json:"disposition"`
	} `json:"streams"`
	Format struct {
		Filename       string `json:"filename"`
		NbStreams      int    `json:"nb_streams"`
		NbPrograms     int    `json:"nb_programs"`
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"format_long_name"`
		Size           string `json:"size"`
		ProbeScore     int    `json:"probe_score"`
	} `json:"format"`
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

func readInt(buf *bytes.Buffer, delim byte) (int, error) {
	s, err := buf.ReadString(delim)
	if err != nil {
		return 0, fmt.Errorf("unable to read: %w", err)
	}
	return strconv.Atoi(s[:len(s)-1])
}

type pamImage struct {
	Width     int
	Height    int
	Depth     int
	MaxValue  int
	TupleType string
	Bytes     []byte
}

func readPAM(b []byte) (pamImage, error) {

	var img pamImage

	if !bytes.HasPrefix(b, pam_prefix_magic) {
		return img, fmt.Errorf("expected magic prefix %v", pam_prefix_magic)
	}

	b = b[len(pam_prefix_magic):]
	header := b[:256]
	buf := bytes.NewBuffer(header)

	for {
		key, err := buf.ReadString(' ')
		if err != nil {
			return img, err
		}
		key = key[:len(key)-1]
		switch key {
		case "WIDTH":
			img.Width, err = readInt(buf, '\n')
		case "HEIGHT":
			img.Height, err = readInt(buf, '\n')
		case "DEPTH":
			img.Depth, err = readInt(buf, '\n')
		case "MAXVAL":
			img.MaxValue, err = readInt(buf, '\n')
		case "TUPLTYPE":
			img.TupleType, err = buf.ReadString('\n')
		default:
			return img, fmt.Errorf("unexpected key: %s", key)
		}
		if err != nil {
			return img, err
		}
		if key == "TUPLTYPE" {
			end := buf.Bytes()
			if !bytes.HasPrefix(end, pam_header_end) {
				return img, fmt.Errorf("expected end of header marker")
			}
			start := len(header) - buf.Len() + len(pam_header_end)
			// println("len", buf.Len(), start)
			// fmt.Printf(">%s<\n", b[start:100])
			img.Bytes = b[start:]
			break
		}
	}
	return img, nil
}
