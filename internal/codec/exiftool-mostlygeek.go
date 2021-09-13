package photofield

import (
	"bufio"
	"errors"
	"log"
	"strconv"
	"strings"

	. "photofield/internal"

	"github.com/mostlygeek/go-exiftool"
)

type ExifToolMostlyGeekDecoder struct {
	exifTool *exiftool.Pool
}

func NewExifToolMostlyGeekDecoder(exifToolCount int) *ExifToolMostlyGeekDecoder {
	if exifToolCount <= 0 {
		return nil
	}
	var err error
	decoder := &ExifToolMostlyGeekDecoder{}
	decoder.exifTool, err = exiftool.NewPool(
		"exiftool", exifToolCount,
		"-Orientation",
		"-Rotation",
		"-ImageWidth",
		"-ImageHeight",
		// First available will be used
		"-SubSecDateTimeOriginal",
		"-DateTimeOriginal",
		"-EXIF:DateTimeOriginal",
		"-CreateDate",
		"-XMP:CreateDate",
		"-EXIF:CreateDate",
		"-XMP:DateTimeOriginal",
		"-GPSDateTime",
		"-TimeStamp",
		"-FileModifyDate",
		"-FileCreateDate",
		"-n", // Machine-readable values
		"-S", // Short tag names with no padding
	)
	if err != nil {
		log.Printf("exiftool error: %v\n", err.Error())
		return nil
	}
	return decoder
}

func (decoder *ExifToolMostlyGeekDecoder) DecodeInfo(path string, info *ImageInfo) error {

	if decoder == nil {
		return errors.New("Unable to decode, exiftool missing")
	}

	bytes, err := decoder.exifTool.Extract(path)
	if err != nil {
		return err
	}

	orientation := ""
	rotation := ""
	imageWidth := ""
	imageHeight := ""

	// var gpsTime time.Time

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
		// case "GPSDateTime":
		// 	gpsTime, _ = parseDateTime(value)
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

	// println(gpsTime.String(), info.DateTime.String())

	// if !gpsTime.IsZero() {
	// time.FixedZone()
	// }

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

func (decoder *ExifToolMostlyGeekDecoder) Close() {
	if decoder.exifTool != nil {
		decoder.exifTool.Stop()
	}
}
