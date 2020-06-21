package photofield

import (
	"bufio"
	"image"
	"io"
	"strconv"
	"strings"
	"time"

	. "photofield/internal"

	"github.com/disintegration/imaging"
	"github.com/mostlygeek/go-exiftool"
	"github.com/rwcarlsen/goexif/exif"
)

type MediaDecoder struct {
	exifTool *exiftool.Pool
}

func NewMediaDecoder(concurrent int) *MediaDecoder {
	var err error
	decoder := MediaDecoder{}
	decoder.exifTool, err = exiftool.NewPool(
		"exiftool", concurrent,
		"-Orientation",
		"-Rotation",
		"-ImageWidth",
		"-ImageHeight",
		"-FileCreateDate", // This likely exists and contains timezone
		"-DateTimeOriginal",
		"-CreateDate",
		"-Time:All",
		"-n", // Machine-readable values
		"-S", // Short tag names with no padding
	)
	if err != nil {
		panic(err)
	}
	return &decoder
}

func (decoder *MediaDecoder) Decode(reader io.ReadSeeker) (image.Image, string, error) {
	img, fmt, err := image.Decode(reader)
	if err != nil {
		return img, fmt, err
	}
	reader.Seek(0, io.SeekStart)
	orientation := getOrientation(reader)
	switch orientation {
	case "1":
	case "2":
		img = imaging.FlipH(img)
	case "3":
		img = imaging.Rotate180(img)
	case "4":
		img = imaging.Rotate180(imaging.FlipH(img))
	case "5":
		img = imaging.Rotate270(imaging.FlipV(img))
	case "6":
		img = imaging.Rotate270(img)
	case "7":
		img = imaging.Rotate90(imaging.FlipV(img))
	case "8":
		img = imaging.Rotate90(img)
	}

	return img, fmt, err
}

func parseDateTime(value string) (time.Time, error) {
	t, err := time.Parse("2006:01:02 15:04:05Z07:00", value)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006:01:02 15:04:05", value)
	if err == nil {
		return t, nil
	}
	return t, err
}

func (decoder *MediaDecoder) DecodeInfoExifTool(path string, info *ImageInfo) error {

	bytes, err := decoder.exifTool.Extract(path)
	if err != nil {
		return err
	}

	orientation := ""
	rotation := ""
	imageWidth := ""
	imageHeight := ""

	output := string(bytes)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		nameValueSplit := strings.SplitN(line, ":", 2)
		if len(nameValueSplit) < 2 {
			continue
		}
		name := strings.TrimSpace(nameValueSplit[0])
		value := strings.TrimSpace(nameValueSplit[1])
		// println(name, value)
		switch name {
		case "Orientation":
			orientation = value
		case "Rotation":
			rotation = value
		case "ImageWidth":
			imageWidth = value
		case "ImageHeight":
			imageHeight = value
		default:
			if info.DateTime.IsZero() &&
				(strings.Contains(name, "Date") || strings.Contains(name, "Time")) {
				info.DateTime, _ = parseDateTime(value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if imageWidth != "" {
		info.Width, err = strconv.Atoi(imageWidth)
		if err != nil {
			info.Width = 0
		}
	}

	if imageHeight != "" {
		info.Height, err = strconv.Atoi(imageHeight)
		if err != nil {
			info.Height = 0
		}
	}

	portrait := false
	if orientation != "" {
		portrait = getPortraitFromOrientation(orientation)
	} else if rotation != "" {
		portrait = getPortraitFromRotation(rotation)
	}

	if portrait {
		info.Width, info.Height = info.Height, info.Width
	}

	// println(path, info.Width, info.Height, info.DateTime.String())

	return nil
}

func (decoder *MediaDecoder) DecodeInfo(reader io.ReadSeeker, info *ImageInfo) error {

	x, err := exif.Decode(reader)
	if err == nil {
		info.DateTime, _ = x.DateTime()
	}

	portrait := getPortraitFromExif(x)
	reader.Seek(0, io.SeekStart)
	conf, _, err := image.DecodeConfig(reader)
	if err != nil {
		return err
	}
	if portrait {
		conf.Width, conf.Height = conf.Height, conf.Width
	}

	if err != nil {
		return err
	}
	info.Width, info.Height = conf.Width, conf.Height

	return nil
}

func getPortraitFromExif(x *exif.Exif) bool {
	return getPortraitFromOrientation(getOrientationFromExif(x))
}

func getPortraitFromRotation(rotation string) bool {
	switch rotation {
	case "90":
		fallthrough
	case "270":
		return true
	default:
		return false
	}
}

func getPortraitFromOrientation(orientation string) bool {
	switch orientation {
	case "1":
		fallthrough
	case "2":
		fallthrough
	case "3":
		fallthrough
	case "4":
		return false
	case "5":
		fallthrough
	case "6":
		fallthrough
	case "7":
		fallthrough
	case "8":
		return true
	default:
		return false
	}
}

func getOrientationFromExif(x *exif.Exif) string {
	if x == nil {
		return "1"
	}
	orient, err := x.Get(exif.Orientation)
	if err != nil {
		return "1"
	}
	if orient != nil {
		return orient.String()
	}
	return "1"
}

func (decoder *MediaDecoder) DecodeConfig(reader io.ReadSeeker) (image.Config, string, error) {
	conf, fmt, err := image.DecodeConfig(reader)
	if err != nil {
		return conf, fmt, err
	}
	reader.Seek(0, io.SeekStart)
	orientation := getOrientation(reader)
	switch orientation {
	case "5":
		fallthrough
	case "6":
		fallthrough
	case "7":
		fallthrough
	case "8":
		conf.Width, conf.Height = conf.Height, conf.Width
	}
	return conf, fmt, err
}

func getOrientation(reader io.ReadSeeker) string {
	x, err := exif.Decode(reader)
	if err != nil {
		return "1"
	}
	if x != nil {
		orient, err := x.Get(exif.Orientation)
		if err != nil {
			return "1"
		}
		if orient != nil {
			return orient.String()
		}
	}

	return "1"
}
