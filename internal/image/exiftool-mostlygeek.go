package image

import (
	"bufio"
	"errors"
	"math"
	"photofield/internal/tag"
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

func NewExifToolMostlyGeekLoader(exifToolCount int, exifToolPath string) (*ExifToolMostlyGeekLoader, error) {
	if exifToolCount <= 0 {
		return nil, errors.New("invalid exif tool count")
	}

	// Use provided path or fallback to "exiftool"
	path := "exiftool"
	if exifToolPath != "" {
		path = exifToolPath
	}

	var err error
	decoder := &ExifToolMostlyGeekLoader{}
	decoder.exifTool, err = exiftool.NewPool(
		path,
		exifToolCount,
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
		// println(path, name, value)
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
		default:
			if name, ok := tag.ExifTagToName[name]; ok {
				tags = append(tags, tag.NewExif(name, value))
			}
			if strings.Contains(name, "Date") || strings.Contains(name, "Time") {
				// println(path, info.DateTime.IsZero(), name, value)
				if info.DateTime.IsZero() {
					t, _, _, err := parseDateTime(value)
					if err == nil {
						info.DateTime = t
					}
				} else if name != "GPSDateTime" && name != "FileModifyDate" && name != "FileCreateDate" {
					t, hasTimezone, _, err := parseDateTime(value)
					if err == nil && info.DateTime.Location() == time.UTC {
						if hasTimezone {
							// Prefer time with timezone if available
							info.DateTime = t
						} else {
							// If there are two times that are more than 10 minutes apart and
							// the first one doesn't have a timezone, it's likely that the
							// first one is local time and the second one is UTC, so we add
							// the timezone to the first one.
							d := info.DateTime.Sub(t)
							if d.Abs() > 10*time.Minute {
								d = d.Truncate(time.Minute)
								info.DateTime = info.DateTime.Add(-d).In(time.FixedZone("", int(d.Seconds())))
							}
						}
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
