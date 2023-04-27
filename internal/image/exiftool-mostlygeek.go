package image

import (
	"bufio"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"math"

	"github.com/mostlygeek/go-exiftool"
)

var previewValueMatcher = regexp.MustCompile(`Binary data (\d+) bytes`)

type ExifToolMostlyGeekLoader struct {
	exifTool *exiftool.Pool
}

func NewExifToolMostlyGeekLoader(exifToolCount int) (*ExifToolMostlyGeekLoader, error) {
	if exifToolCount <= 0 {
		return nil, errors.New("invalid exif tool count")
	}
	var err error
	decoder := &ExifToolMostlyGeekLoader{}
	decoder.exifTool, err = exiftool.NewPool(
		"exiftool", exifToolCount,
		"-n", // Machine-readable values
		"-S", // Short tag names with no padding
	)
	return decoder, err
}

func (decoder *ExifToolMostlyGeekLoader) DecodeInfo(path string, info *Info) error {

	if decoder == nil {
		return errors.New("unable to decode, exiftool missing")
	}

	bytes, err := decoder.exifTool.ExtractFlags(path,
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
		// Location Info
		"-GPSLatitude",
		"-GPSLongitude",
	)
	if err != nil {
		return err
	}

	orientation := ""
	rotation := ""
	imageWidth := ""
	imageHeight := ""
	latitude := ""
	longitude := ""

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
		case "GPSLatitude":
			latitude = value
		case "GPSLongitude":
			longitude = value
		
		// case "GPSDateTime":
		// 	gpsTime, _ = parseDateTime(value)
		default:
			if strings.Contains(name, "Date") || strings.Contains(name, "Time") {
				if info.DateTime.IsZero() {
					info.DateTime, _, _, _ = parseDateTime(value)
				} else if name != "FileModifyDate" && name != "FileCreateDate" {
					// Prefer time with timezone if available
					t, hasTimezone, _, _ := parseDateTime(value)
					if hasTimezone && info.DateTime.Location() == time.UTC {
						info.DateTime = t
					}
				}
			} else if strings.HasSuffix(name, "Image") {
				match := previewValueMatcher.FindStringSubmatch(value)
				if len(match) >= 2 {
					println(name, match[1])
				}
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

	if orientation != "" {
		info.Orientation = parseOrientation(orientation)
	} else if rotation != "" {
		info.Orientation = getOrientationFromRotation(rotation)
	}

	if latitude != "" {
		info.Latitude, err = strconv.ParseFloat(latitude, 64)
		if err != nil {
			info.Latitude = math.NaN()
		}
	}

	if longitude != "" {
		info.Longitude, err = strconv.ParseFloat(longitude, 64)
		if err != nil {
			info.Longitude = math.NaN()
		}
	}

	if info.Orientation.SwapsDimensions() {
		info.Width, info.Height = info.Height, info.Width
	}

	return nil
}

func (decoder *ExifToolMostlyGeekLoader) DecodeBytes(path string, tagName string) ([]byte, error) {

	bytes, err := decoder.exifTool.ExtractFlags(path, "-b", "-"+tagName)

	if err != nil {
		println(path, tagName, err.Error())
		return nil, err
	}

	return bytes, nil
}

func (decoder *ExifToolMostlyGeekLoader) Close() {
	if decoder.exifTool != nil {
		decoder.exifTool.Stop()
	}
}
