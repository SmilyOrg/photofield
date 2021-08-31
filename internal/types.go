package photofield

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"text/template"
	"time"

	"github.com/docker/go-units"
)

var MetricsNamespace = "pf"

type TileRequestConfig struct {
	Concurrency int  `json:"concurrency"`
	LogStats    bool `json:"log_stats"`
}

type CacheConfig struct {
	MaxSize string `json:"max_size"`
}

func (config *CacheConfig) MaxSizeBytes() int64 {
	value, err := units.FromHumanSize(config.MaxSize)
	if err != nil {
		panic(err)
	}
	return value
}

type Caches struct {
	Image CacheConfig
}

type System struct {
	ExifToolCount int    `json:"exif_tool_count"`
	SkipLoadInfo  bool   `json:"skip_load_info"`
	Caches        Caches `json:"caches"`
}

type Size image.Point

type ImageInfo struct {
	Width, Height int
	DateTime      time.Time
	Color         uint32
}

func (info *ImageInfo) IsZero() bool {
	return info.Width == 0 &&
		info.Height == 0 &&
		info.DateTime.IsZero() &&
		info.Color == 0
}

func (info *ImageInfo) HasMeta() bool {
	return info.Width != 0 ||
		info.Height != 0 ||
		!info.DateTime.IsZero()
}

func (info *ImageInfo) NeedsMeta() bool {
	return info.Width == 0 ||
		info.Height == 0 ||
		info.DateTime.IsZero()
}

func (info *ImageInfo) NeedsColor() bool {
	return info.Color == 0
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
	FitOutside   ThumbnailSizeType = iota
	FitInside    ThumbnailSizeType = iota
	OriginalSize ThumbnailSizeType = iota
)

type ThumbnailSize struct {
}

type Thumbnail struct {
	Name            string `json:"name"`
	PathTemplateRaw string `json:"path"`
	PathTemplate    *template.Template

	SizeTypeRaw string `json:"fit"`
	SizeType    ThumbnailSizeType

	Width  int `json:"width"`
	Height int `json:"height"`
}

func (thumbnail *Thumbnail) Init() {
	var err error
	thumbnail.PathTemplate, err = template.New("").Parse(thumbnail.PathTemplateRaw)
	if err != nil {
		panic(err)
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
	return Size{
		X: int(math.Round(thumbWidth)),
		Y: int(math.Round(thumbHeight)),
	}
}
