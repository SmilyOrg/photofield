package photofield

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"text/template"
	"time"
)

type Size image.Point

type ImageInfo struct {
	Width, Height int
	DateTime      time.Time
	Color         uint32
}

func (info *ImageInfo) GetColor() color.RGBA {
	return color.RGBA{
		A: uint8((info.Color >> 24) & 0xFF),
		R: uint8((info.Color >> 16) & 0xFF),
		G: uint8((info.Color >> 8) & 0xFF),
		B: uint8(info.Color & 0xFF),
	}
}

func (info *ImageInfo) SetColorRGBA(color color.RGBA) {
	info.Color = (uint32(color.A&0xFF) << 24) |
		(uint32(color.R&0xFF) << 16) |
		(uint32(color.G&0xFF) << 8) |
		uint32(color.B&0xFF)
}

func (info *ImageInfo) SetColorRGB32(r uint32, g uint32, b uint32) {
	info.Color = (uint32(0xFF) << 24) |
		(uint32(r&0xFF) << 16) |
		(uint32(g&0xFF) << 8) |
		uint32(b&0xFF)
}

type ThumbnailSizeType int32

const (
	FitOutside ThumbnailSizeType = iota
	FitInside  ThumbnailSizeType = iota
)

type Thumbnail struct {
	Name         string
	PathTemplate *template.Template
	SizeType     ThumbnailSizeType
	Size         Size
}

func NewThumbnail(name string, pathTemplate string, sizeType ThumbnailSizeType, size Size) Thumbnail {
	template, err := template.New("").Parse(pathTemplate)
	if err != nil {
		panic(err)
	}
	return Thumbnail{
		Name:         name,
		PathTemplate: template,
		SizeType:     sizeType,
		Size:         size,
	}
}

type PhotoTemplateData struct {
	Dir      string
	Filename string
}

func (thumbnail *Thumbnail) GetPath(originalPath string) string {
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

func (thumbnail *Thumbnail) Fit(originalSize Size) Size {
	thumbWidth, thumbHeight := float64(thumbnail.Size.X), float64(thumbnail.Size.Y)
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
	}
	return Size{
		X: int(math.Round(thumbWidth)),
		Y: int(math.Round(thumbHeight)),
	}
}
