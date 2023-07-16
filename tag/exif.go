package tag

import (
	"fmt"
	"strconv"
	"unicode"

	"github.com/gosimple/slug"
)

var exifNames = []string{
	"Make",
	"Model",
	// "ISO",
	// "ShutterSpeed",
	// "Aperture",
	// "ExposureCompensation",
	// "FocalLength35efl",
	// "FocusMode",
	// "WhiteBalance",
	// "MeteringMode",
	// "SelfTimer",
}

var exifSlugs []string
var ExifTagToName = map[string]string{}
var ExifFlags []string

func init() {
	for _, name := range exifNames {
		slug := pascalCaseToKebabCase(name)
		exifSlugs = append(exifSlugs, slug)
		ExifTagToName[name] = slug
		ExifFlags = append(ExifFlags, fmt.Sprintf("-%s", name))
	}
}

func pascalCaseToKebabCase(s string) string {
	var result []rune
	lastUpper := false
	lastDigit := false
	for i, r := range s {
		upper := unicode.IsUpper(r)
		digit := unicode.IsDigit(r)
		if upper || digit {
			if i > 0 && !lastUpper && !lastDigit {
				result = append(result, '-')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
		lastUpper = upper
		lastDigit = digit
	}
	return string(result)
}

func NewExif(name string, value string) Tag {
	var t Tag
	v := ""
	_, err := strconv.ParseFloat(value, 64)
	if err == nil {
		v = value
	} else {
		v = slug.Make(value)
	}
	t.Name = fmt.Sprintf("exif:%s:%s", name, v)
	return t
}
