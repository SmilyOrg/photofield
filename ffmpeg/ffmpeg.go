package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"os/exec"
	"strconv"
	"time"
)

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

type ForceOriginalAspectRatio string

const (
	Decrease ForceOriginalAspectRatio = "decrease"
	Increase ForceOriginalAspectRatio = "increase"
)

func Decode(ctx context.Context, path string, maxWidth int, maxHeight int, foar ForceOriginalAspectRatio) (image.Image, image.Config, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// cmd := exec.CommandContext(
	// 	ctx,
	// 	"ffprobe",
	// 	"-v", "quiet",
	// 	"-print_format", "json",
	// 	"-show_format",
	// 	"-show_streams",
	// 	path,
	// )
	// bytes, err := cmd.Output()
	// err = formatErr(err, "ffprobe")
	// if err != nil {
	// 	return nil, "", err
	// }

	// var m metadata
	// json.Unmarshal(bytes, &m)
	// if len(m.Streams) < 1 {
	// 	return nil, "", fmt.Errorf("missing streams")
	// }

	// stream := m.Streams[0]
	// w, h := stream.Width, stream.Height

	var c image.Config

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-i", path,
		"-vframes", "1",
		"-vf", fmt.Sprintf("scale='min(iw,%d)':'min(ih,%d)':force_original_aspect_ratio=%s", maxWidth, maxHeight, foar),
		// "-q:v", "2",
		// "-f", "image2pipe", // jpeg
		"-c:v", "pam",
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-an", // no audio
		"-",
	)
	b, err := cmd.Output()
	err = formatErr(err, "ffmpeg")
	if err != nil {
		return nil, c, err
	}

	pam, err := readPAM(b)
	if err != nil {
		return nil, c, err
	}

	if pam.Depth != 4 {
		return nil, c, fmt.Errorf("unexpected depth %d", pam.Depth)
	}

	if pam.MaxValue != 255 {
		return nil, c, fmt.Errorf("unexpected max value %d", pam.MaxValue)
	}

	if pam.Width < 0 || pam.Height < 0 {
		return nil, c, fmt.Errorf("unexpected size %d x %d", pam.Width, pam.Height)
	}

	c.Width = pam.Width
	c.Height = pam.Height
	c.ColorModel = color.RGBAModel

	rgba := image.RGBA{
		Pix:    pam.Bytes,
		Stride: 4 * pam.Width,
		Rect:   image.Rect(0, 0, pam.Width, pam.Height),
	}
	return image.Image(&rgba), c, nil
}
