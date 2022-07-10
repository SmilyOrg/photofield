package image

import (
	"bytes"
	"image"
	"math"
	"path/filepath"
	"text/template"
)

type PhotoTemplateData struct {
	Dir      string
	Filename string
}

type Thumbnail struct {
	Name            string `json:"name"`
	PathTemplateRaw string `json:"path"`
	PathTemplate    *template.Template
	Exif            string   `json:"exif"`
	Extensions      []string `json:"extensions"`

	SizeTypeRaw string `json:"fit"`
	SizeType    ThumbnailSizeType

	Width     int `json:"width"`
	Height    int `json:"height"`
	ExtraCost int `json:"extra_cost"`
}

type ThumbnailSizeType int32

const (
	FitOutside   ThumbnailSizeType = iota
	FitInside    ThumbnailSizeType = iota
	OriginalSize ThumbnailSizeType = iota
)

func (thumbnail *Thumbnail) Init() {
	if thumbnail.PathTemplateRaw != "" {
		var err error
		thumbnail.PathTemplate, err = template.New("").Parse(thumbnail.PathTemplateRaw)
		if err != nil {
			panic(err)
		}
	} else if thumbnail.Exif != "" {
		// No setup required
	} else {
		panic("thumbnail path or exif name must be specified")
	}

	switch thumbnail.SizeTypeRaw {
	case "INSIDE":
		thumbnail.SizeType = FitInside
	case "OUTSIDE":
		thumbnail.SizeType = FitOutside
	case "ORIGINAL":
		thumbnail.SizeType = OriginalSize
	default:
		panic("Unsupported thumbnail fit: " + thumbnail.SizeTypeRaw)
	}
}

func (thumbnail *Thumbnail) GetPath(originalPath string) string {
	if thumbnail.PathTemplate == nil {
		return ""
	}
	var rendered bytes.Buffer
	dir, filename := filepath.Split(originalPath)
	err := thumbnail.PathTemplate.Execute(&rendered, PhotoTemplateData{
		Dir:      dir,
		Filename: filename,
	})
	if err != nil {
		panic(err)
	}
	return rendered.String()
}

func (thumbnail *Thumbnail) Fit(originalSize image.Point) image.Point {
	thumbWidth, thumbHeight := float64(thumbnail.Width), float64(thumbnail.Height)
	thumbRatio := thumbWidth / thumbHeight
	originalWidth, originalHeight := float64(originalSize.X), float64(originalSize.Y)
	originalRatio := originalWidth / originalHeight
	switch thumbnail.SizeType {
	case FitInside:
		if thumbRatio < originalRatio {
			thumbHeight = thumbWidth / originalRatio
		} else {
			thumbWidth = thumbHeight * originalRatio
		}
	case FitOutside:
		if thumbRatio > originalRatio {
			thumbHeight = thumbWidth / originalRatio
		} else {
			thumbWidth = thumbHeight * originalRatio
		}
	case OriginalSize:
		thumbWidth = originalWidth
		thumbHeight = originalHeight
	}
	return image.Point{
		X: int(math.Round(thumbWidth)),
		Y: int(math.Round(thumbHeight)),
	}
}
