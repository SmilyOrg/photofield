package image

import (
	"embed"
	"fmt"
	"path/filepath"
	"photofield/io"
	"photofield/io/cached"
	"photofield/io/ffmpeg"
	"photofield/io/filtered"
	"photofield/io/goexif"
	"photofield/io/goimage"
	"photofield/io/ristretto"
	"photofield/io/sqlite"
	"photofield/io/thumb"
	"strings"

	"github.com/goccy/go-yaml"
)

const (
	SourceTypeNone   = ""
	SourceTypeSqlite = "SQLITE"
	SourceTypeGoexif = "GOEXIF"
	SourceTypeThumb  = "THUMB"
	SourceTypeImage  = "IMAGE"
	SourceTypeFFmpeg = "FFMPEG"
)

// SourceType is the type of a source (e.g. SQLITE, THUMB, IMAGE, FFMPEG)
type SourceType string

func (t *SourceType) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	*t = SourceType(strings.ToUpper(s))
	return nil
}

type SourceConfig struct {
	Name       string            `json:"name"`
	Type       SourceType        `json:"type"`
	Path       string            `json:"path"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Fit        io.AspectRatioFit `json:"fit"`
	Extensions []string          `json:"extensions"`
}

type ThumbnailConfig struct {
	Sources    SourceConfigs `json:"sources"`
	Generators SourceConfigs `json:"generators"`
	Sink       SourceConfig  `json:"sink"`
}

// SourceEnvironment is the environment for creating sources
type SourceEnvironment struct {
	DataDir    string
	FFmpegPath string
	Migrations embed.FS
	ImageCache *ristretto.Ristretto
	Databases  map[string]*sqlite.Source
}

func (c SourceConfig) NewSource(env *SourceEnvironment) (io.Source, error) {
	switch c.Type {

	case SourceTypeSqlite:
		existing, ok := env.Databases[c.Path]
		if ok {
			return existing, nil
		}
		s := sqlite.New(
			filepath.Join(env.DataDir, c.Path),
			env.Migrations,
		)
		if env.Databases == nil {
			env.Databases = make(map[string]*sqlite.Source)
		}
		env.Databases[c.Path] = s
		return s, nil

	case SourceTypeGoexif:
		return goexif.Exif{
			Width:  c.Width,
			Height: c.Height,
		}, nil

	case SourceTypeThumb:
		return thumb.New(
			c.Name,
			c.Path,
			c.Fit,
			c.Width,
			c.Height,
		), nil

	case SourceTypeImage:
		return goimage.Image{
			Width:  c.Width,
			Height: c.Height,
		}, nil

	case SourceTypeFFmpeg:
		return ffmpeg.FFmpeg{
			Path:   env.FFmpegPath,
			Width:  c.Width,
			Height: c.Height,
			Fit:    c.Fit,
		}, nil

	default:
		return nil, fmt.Errorf("unknown source type: %s", c.Type)
	}
}

type SourceConfigs []SourceConfig

// NewSources creates a list of sources from the configuration
// and adds caching and filtering layers if needed
func (cfgs SourceConfigs) NewSources(env *SourceEnvironment) ([]io.Source, error) {
	var sources []io.Source
	for _, c := range cfgs {
		s, err := c.NewSource(env)
		if err != nil {
			return nil, err
		}
		if env.ImageCache != nil {
			// Add caching layer
			s = &cached.Cached{
				Source: s,
				Cache:  *env.ImageCache,
			}
		}
		// Add filtering layer
		if len(c.Extensions) > 0 {
			s = &filtered.Filtered{
				Source:     s,
				Extensions: c.Extensions,
			}
		}
		sources = append(sources, s)
	}
	return sources, nil
}
