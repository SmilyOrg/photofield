package ffmpeg

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"log"
	"os/exec"
	"photofield/io"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

func (f FFmpeg) Close() error {
	return nil
}

func (f FFmpeg) DisplayName() string {
	return "FFmpeg JPEG"
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

func (f FFmpeg) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return true
}

func (f FFmpeg) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	if f.Path == "" {
		return io.Result{Error: ErrMissingBinary}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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
	r := f.Get(ctx, id, path)
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

func (f FFmpeg) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}

func (f FFmpeg) StreamFrames(ctx context.Context, path string, opts io.StreamOptions) <-chan io.FrameResult {
	frames := make(chan io.FrameResult, 4)

	if f.Path == "" {
		frames <- io.FrameResult{Error: ErrMissingBinary}
		close(frames)
		return frames
	}

	go func() {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()

		defer close(frames)

		filterGraph := fmt.Sprintf(
			"fps=1,select='gt(scene,%f)',showinfo,scale='min(iw,%d)':'min(ih,%d)':force_original_aspect_ratio=%s",
			opts.Threshold, f.Width, f.Height, f.ForceOriginalAspectRatio(),
		)

		// Build FFmpeg command for scene detection
		cmd := exec.CommandContext(
			ctx,
			f.Path,
			"-hwaccel", "auto",
			"-hide_banner",
			"-i", path,
			"-vf", filterGraph,
			"-fps_mode", "vfr",
			"-c:v", "mjpeg", // Use JPEG format instead of PAM
			"-f", "image2pipe", // Pipe images
			"-q:v", "2", // JPEG quality (2 is high quality)
			"-an", // No audio
			"-",   // Output to stdout
		)

		// Create pipes for stdout and stderr
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			frames <- io.FrameResult{Error: fmt.Errorf("failed to create stdout pipe: %w", err)}
			return
		}

		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			frames <- io.FrameResult{Error: fmt.Errorf("failed to create stderr pipe: %w", err)}
			return
		}

		if err := cmd.Start(); err != nil {
			frames <- io.FrameResult{Error: formatErr(err, "ffmpeg start")}
			return
		}

		// Setup synchronization for the frame metadata and images
		var wg sync.WaitGroup
		metadataCh := make(chan frameMetadata)

		// Parse stderr in a separate goroutine to extract timestamps
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(metadataCh)
			parseSceneMetadata(stderrPipe, metadataCh)
		}()

		for {
			// Read the JPEG image
			jpegData, err := extractJPEG(stdoutPipe)
			if err != nil {
				if err != goio.EOF {
					frames <- io.FrameResult{Error: fmt.Errorf("error reading JPEG image: %w", err)}
				}
				break
			}

			meta, ok := <-metadataCh
			if !ok {
				// No more metadata, so we're done
				break
			}

			// Send the frame to the output channel
			select {
			case frames <- io.FrameResult{
				Bytes:        jpegData,
				FrameNumber:  meta.ShowInfoNum,
				TimestampSec: meta.PtsTime,
				Type:         meta.FrameType,
				IsKeyframe:   meta.IsKey,
			}:
			case <-ctx.Done():
				frames <- io.FrameResult{Error: ctx.Err()}
				return
			}
		}

		// Wait for the metadata parsing goroutine to finish
		wg.Wait()

		if err := cmd.Wait(); err != nil {
			// Only report error if it's not due to context cancellation
			select {
			case <-ctx.Done():
				frames <- io.FrameResult{Error: ctx.Err()}
			default:
				frames <- io.FrameResult{Error: formatErr(err, "ffmpeg stream")}
			}
		}
	}()

	return frames
}

// ExtractJPEG reads a stream until it finds a complete JPEG image (SOI to EOI)
// It returns the JPEG bytes or an error (io.EOF when end of stream is reached)
func extractJPEG(r goio.Reader) ([]byte, error) {
	reader := bufio.NewReader(r)
	var buffer bytes.Buffer
	var prevByte byte
	inImage := false

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err // Return EOF or other error
		}

		// Check for SOI marker (0xFF 0xD8)
		if !inImage && prevByte == 0xFF && b == 0xD8 {
			buffer.Reset()
			buffer.Write([]byte{0xFF, 0xD8})
			inImage = true
		} else if inImage {
			buffer.WriteByte(b)

			// Check for EOI marker (0xFF 0xD9)
			if prevByte == 0xFF && b == 0xD9 {
				return buffer.Bytes(), nil
			}
		}

		prevByte = b
	}
}

// func (f FFmpeg) StreamFramesPAM(ctx context.Context, path string, opts io.StreamOptions) (<-chan io.FrameResult, <-chan error) {
// 	frames := make(chan io.FrameResult)
// 	errCh := make(chan error, 1) // Buffer size 1 to avoid blocking

// 	if f.Path == "" {
// 		errCh <- ErrMissingBinary
// 		close(frames)
// 		close(errCh)
// 		return frames, errCh
// 	}

// 	go func() {
// 		defer close(frames)
// 		defer close(errCh)

// 		filterGraph := fmt.Sprintf(
// 			"fps=1,select='gt(scene,%f)',showinfo,scale='min(iw,%d)':'min(ih,%d)':force_original_aspect_ratio=%s",
// 			opts.Threshold, f.Width, f.Height, f.ForceOriginalAspectRatio(),
// 		)

// 		// Build FFmpeg command for scene detection
// 		cmd := exec.CommandContext(
// 			ctx,
// 			f.Path,
// 			"-hwaccel", "auto",
// 			"-hide_banner",
// 			"-i", path,
// 			"-vf", filterGraph,
// 			"-fps_mode", "vfr",
// 			"-c:v", "pam", // Use PAM format for easy parsing
// 			"-f", "image2pipe", // Pipe images
// 			"-pix_fmt", "rgba", // RGBA format
// 			"-an", // No audio
// 			"-",   // Output to stdout
// 		)

// 		// Create pipes for stdout and stderr
// 		stdoutPipe, err := cmd.StdoutPipe()
// 		if err != nil {
// 			errCh <- fmt.Errorf("failed to create stdout pipe: %w", err)
// 			return
// 		}

// 		stderrPipe, err := cmd.StderrPipe()
// 		if err != nil {
// 			errCh <- fmt.Errorf("failed to create stderr pipe: %w", err)
// 			return
// 		}

// 		if err := cmd.Start(); err != nil {
// 			errCh <- formatErr(err, "ffmpeg scene detection")
// 			return
// 		}

// 		// Setup synchronization for the frame metadata and images
// 		var wg sync.WaitGroup
// 		metadataCh := make(chan frameMetadata)

// 		// Parse stderr in a separate goroutine to extract timestamps
// 		wg.Add(1)
// 		go func() {
// 			defer wg.Done()
// 			defer close(metadataCh)
// 			parseSceneMetadata(stderrPipe, metadataCh)
// 		}()

// 		// Read the PAM images from stdout
// 		frameIndex := 0
// 		frameBuf := bytes.NewBuffer(nil)

// 		for {
// 			// Clear the buffer for the next frame
// 			frameBuf.Reset()

// 			// Find the PAM header
// 			header := make([]byte, len(pam_prefix_magic))
// 			_, err := goio.ReadFull(stdoutPipe, header)
// 			if err != nil {
// 				if err != goio.EOF {
// 					errCh <- fmt.Errorf("error reading PAM header: %w", err)
// 				}
// 				break
// 			}

// 			if !bytes.Equal(header, pam_prefix_magic) {
// 				errCh <- fmt.Errorf("invalid PAM header: %v", header)
// 				break
// 			}

// 			frameBuf.Write(header)

// 			// Read until ENDHDR\n
// 			for {
// 				b := make([]byte, 1)
// 				_, err := stdoutPipe.Read(b)
// 				if err != nil {
// 					if err != goio.EOF {
// 						errCh <- fmt.Errorf("error reading PAM header: %w", err)
// 					}
// 					break
// 				}

// 				frameBuf.Write(b)

// 				// Check if we've reached the end of the header
// 				if frameBuf.Len() >= len(pam_header_end) && bytes.HasSuffix(frameBuf.Bytes(), pam_header_end) {
// 					break
// 				}
// 			}

// 			// Parse header to get dimensions
// 			headerStr := frameBuf.String()
// 			width, height, depth, maxVal := parsePAMHeader(headerStr)

// 			if width <= 0 || height <= 0 || depth != 4 || maxVal != 255 {
// 				errCh <- fmt.Errorf("invalid PAM dimensions: %dx%d, depth=%d, maxVal=%d",
// 					width, height, depth, maxVal)
// 				break
// 			}

// 			// Read the image data
// 			imageData := make([]byte, width*height*depth)
// 			_, err = goio.ReadFull(stdoutPipe, imageData)
// 			if err != nil {
// 				if err != goio.EOF {
// 					errCh <- fmt.Errorf("error reading PAM data: %w", err)
// 				}
// 				break
// 			}

// 			// Create the RGBA image
// 			rgba := &image.RGBA{
// 				Pix:    imageData,
// 				Stride: width * 4,
// 				Rect:   image.Rect(0, 0, width, height),
// 			}

// 			// Wait for metadata for this frame
// 			meta, ok := <-metadataCh
// 			if !ok {
// 				// No more metadata, so we're done
// 				break
// 			}

// 			// Send the frame to the output channel
// 			select {
// 			case frames <- io.FrameResult{
// 				FrameNumber:  meta.ShowInfoNum,
// 				TimestampSec: meta.PtsTime,
// 				Type:         meta.FrameType,
// 				IsKeyframe:   meta.IsKey,
// 				Result: io.Result{
// 					Image: image.Image(rgba),
// 				},
// 			}:
// 			case <-ctx.Done():
// 				errCh <- ctx.Err()
// 				return
// 			}

// 			frameIndex++
// 		}

// 		// Wait for the metadata parsing goroutine to finish
// 		wg.Wait()

// 		if err := cmd.Wait(); err != nil {
// 			// Only report error if it's not due to context cancellation
// 			select {
// 			case <-ctx.Done():
// 				errCh <- ctx.Err()
// 			default:
// 				errCh <- formatErr(err, "ffmpeg scene detection")
// 			}
// 		}
// 	}()

// 	return frames, errCh
// }

type frameMetadata struct {
	ShowInfoNum int
	Pts         int
	PtsTime     int
	IsKey       bool
	FrameType   string
}

// Regular expression to match showinfo output lines
var showInfoRegex = regexp.MustCompile(`\[Parsed_showinfo_\d+ @ [^\]]+\] n:\s+(\d+) pts:\s+(\d+) pts_time:([+-]?([0-9]*[.])?[0-9]+)\s+.*iskey:(\d+) type:(\w+)`)

// parseSceneMetadata parses the stderr output to extract frame metadata
func parseSceneMetadata(stderr goio.Reader, metadataCh chan<- frameMetadata) {
	scanner := bufio.NewScanner(stderr)

	for scanner.Scan() {
		line := scanner.Text()
		// fmt.Printf("ffmpeg: %s\n", line)
		matches := showInfoRegex.FindStringSubmatch(line)

		if len(matches) >= 6 {
			showInfoNum, _ := strconv.Atoi(matches[1])
			pts, _ := strconv.Atoi(matches[2])
			ptsTime, _ := strconv.Atoi(matches[3])
			isKey, _ := strconv.Atoi(matches[4])
			frameType := matches[5]

			metadataCh <- frameMetadata{
				ShowInfoNum: showInfoNum,
				Pts:         pts,
				PtsTime:     ptsTime,
				IsKey:       isKey == 1,
				FrameType:   frameType,
			}
		}
	}
}

// parsePAMHeader extracts image dimensions from PAM header
func parsePAMHeader(header string) (width, height, depth, maxVal int) {
	lines := strings.Split(header, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "WIDTH ") {
			width, _ = strconv.Atoi(strings.TrimPrefix(line, "WIDTH "))
		} else if strings.HasPrefix(line, "HEIGHT ") {
			height, _ = strconv.Atoi(strings.TrimPrefix(line, "HEIGHT "))
		} else if strings.HasPrefix(line, "DEPTH ") {
			depth, _ = strconv.Atoi(strings.TrimPrefix(line, "DEPTH "))
		} else if strings.HasPrefix(line, "MAXVAL ") {
			maxVal, _ = strconv.Atoi(strings.TrimPrefix(line, "MAXVAL "))
		}
	}

	return width, height, depth, maxVal
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
