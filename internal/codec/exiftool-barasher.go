package photofield

import (
	"errors"
	"log"
	"strings"

	. "photofield/internal"

	"github.com/barasher/go-exiftool"
)

type ExifToolBarasherDecoder struct {
	exifTool *exiftool.Exiftool
}

func NewExifToolBarasherDecoder(exifToolCount int) *ExifToolBarasherDecoder {
	var err error
	decoder := &ExifToolBarasherDecoder{}
	decoder.exifTool, err = exiftool.NewExiftool(exiftool.ExtraInitArgs([]string{"-n"}))
	if err != nil {
		log.Printf("exiftool error: %v\n", err.Error())
		return nil
	}
	return decoder
}

func (decoder *ExifToolBarasherDecoder) DecodeInfo(path string, info *ImageInfo) error {

	if decoder == nil {
		return errors.New("Unable to decode, exiftool missing")
	}

	fileInfo := decoder.exifTool.ExtractMetadata(path)[0]

	// var err error

	orientation := ""
	rotation := ""

	for name, _ := range fileInfo.Fields {

		// fmt.Printf("[%v] %v\n", name, value)

		switch name {
		case "Orientation":
			v, err := fileInfo.GetString(name)
			if err == nil {
				orientation = v
			}
		case "Rotation":
			v, err := fileInfo.GetString(name)
			if err == nil {
				rotation = v
			}
		case "ImageWidth":
			v, err := fileInfo.GetInt(name)
			if err == nil {
				info.Width = int(v)
			}
		case "ImageHeight":
			v, err := fileInfo.GetInt(name)
			if err == nil {
				info.Height = int(v)
			}
		// case "GPSDateTime":
		// 	gpsTime, _ = parseDateTime(value)
		default:
			if info.DateTime.IsZero() &&
				(strings.Contains(name, "Date") || strings.Contains(name, "Time")) {
				v, err := fileInfo.GetString(name)
				if err == nil {
					info.DateTime, _ = parseDateTime(v)
				}
			}
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

func (decoder *ExifToolBarasherDecoder) Close() {
	if decoder.exifTool != nil {
		println("Closing...")
		decoder.exifTool.Close()
	}
}
