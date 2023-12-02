package image

import (
	"bufio"
	"errors"
	"math"
	"photofield/tag"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/geo/s2"
	"github.com/mostlygeek/go-exiftool"
)

var previewValueMatcher = regexp.MustCompile(`Binary data (\d+) bytes`)

type ExifToolMostlyGeekLoader struct {
	exifTool *exiftool.Pool
	flags    []string
}

func NewExifToolMostlyGeekLoader(exifToolCount int) (*ExifToolMostlyGeekLoader, error) {
	if exifToolCount <= 0 {
		return nil, errors.New("invalid exif tool count")
	}
	var err error
	decoder := &ExifToolMostlyGeekLoader{}
	decoder.exifTool, err = exiftool.NewPool(
		"exiftool", exifToolCount,
		"-S", // Short tag names with no padding
	)

	decoder.flags = append(decoder.flags,
		"-Orientation#",
		"-Rotation#",
		"-ImageWidth#",
		"-ImageHeight#",
	)
	decoder.flags = append(decoder.flags, tag.ExifFlags...)
	decoder.flags = append(decoder.flags,
		// First available will be used
		"-SubSecDateTimeOriginal#",
		"-DateTimeOriginal#",
		"-EXIF:DateTimeOriginal#",
		"-CreateDate#",
		"-XMP:CreateDate#",
		"-EXIF:CreateDate#",
		"-XMP:DateTimeOriginal#",
		"-GPSDateTime#",
		"-TimeStamp#",
		"-FileModifyDate#",
		"-FileCreateDate#",
		// Location Info
		"-GPSLatitude#",
		"-GPSLongitude#",
	)
	return decoder, err
}

func (decoder *ExifToolMostlyGeekLoader) DecodeInfo(path string, info *Info) ([]tag.Tag, error) {

	if decoder == nil {
		return nil, errors.New("unable to decode, exiftool missing")
	}

	bytes, err := decoder.exifTool.ExtractFlags(path, decoder.flags...)
	if err != nil {
		return nil, err
	}

	tags := make([]tag.Tag, 0)
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
			if name, ok := tag.ExifTagToName[name]; ok {
				tags = append(tags, tag.NewExif(name, value))
			}
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
		return tags, err
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

	lat := math.NaN()
	lng := math.NaN()
	if latitude != "" && longitude != "" {
		lat, err = strconv.ParseFloat(latitude, 64)
		if err != nil {
			lat = math.NaN()
		}

		lng, err = strconv.ParseFloat(longitude, 64)
		if err != nil {
			lng = math.NaN()
		}
	}

	if !math.IsNaN(lat) && !math.IsNaN(lng) {
		info.LatLng = s2.LatLngFromDegrees(lat, lng)
	} else {
		info.LatLng = NaNLatLng()
	}

	if info.Orientation.SwapsDimensions() {
		info.Width, info.Height = info.Height, info.Width
	}

	// println(path, info.Width, info.Height, info.DateTime.String())

	return tags, nil
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
		decoder.exifTool = nil
	}
}
