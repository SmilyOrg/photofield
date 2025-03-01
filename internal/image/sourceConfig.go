package image

import (
	"embed"
	"fmt"
	"path/filepath"
	"photofield/io"
	"photofield/io/cached"
	"photofield/io/configured"
	"photofield/io/djpeg"
	"photofield/io/ffmpeg"
	"photofield/io/filtered"
	"photofield/io/goexif"
	"photofield/io/goimage"
	"photofield/io/ristretto"
	"photofield/io/sqlite"
	"photofield/io/thumb"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/imdario/mergo"
)

const (
	SourceTypeNone   = ""
	SourceTypeSqlite = "SQLITE"
	SourceTypeGoexif = "GOEXIF"
	SourceTypeThumb  = "THUMB"
	SourceTypeImage  = "IMAGE"
	SourceTypeFFmpeg = "FFMPEG"
	SourceTypeDjpeg  = "DJPEG"
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
	Cost       configured.Cost   `json:"cost"`
	Type       SourceType        `json:"type"`
	Path       string            `json:"path"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Scale      string            `json:"scale"`
	Fit        io.AspectRatioFit `json:"fit"`
	Extensions []string          `json:"extensions"`
}

type SourceTypeMap map[SourceType]SourceConfig

func (smt *SourceTypeMap) UnmarshalYAML(b []byte) error {
	var m map[SourceType]SourceConfig
	if err := yaml.Unmarshal(b, &m); err != nil {
		return err
	}
	*smt = make(map[SourceType]SourceConfig)
	for st, sc := range m {
		st = SourceType(strings.ToUpper(string(st)))
		sc.Type = st
		(*smt)[st] = sc
	}
	return nil
}

type ThumbnailConfig struct {
	Sources    SourceConfigs `json:"sources"`
	Generators SourceConfigs `json:"generators"`
	Sink       SourceConfig  `json:"sink"`
}

// SourceEnvironment is the environment for creating sources
type SourceEnvironment struct {
	SourceTypes SourceTypeMap
	DataDir     string
	FFmpegPath  string
	DjpegPath   string
	Migrations  embed.FS
	ImageCache  *ristretto.Ristretto
	Databases   map[string]*sqlite.Source
}

func (c SourceConfig) NewSource(env *SourceEnvironment) (io.Source, error) {
	// Merge the source config with the source type config
	if st, ok := env.SourceTypes[c.Type]; ok {
		// println("merging source config with source type config", c.Type, st.Type, st.Cost.Time, st.Cost.TimePerResizedMegapixel, st.Cost.TimePerOriginalMegapixel)
		err := mergo.Merge(&c, &st)
		if err != nil {
			return nil, err
		}
	}

	var s io.Source

	switch c.Type {

	case SourceTypeSqlite:
		existing, ok := env.Databases[c.Path]
		if ok {
			return existing, nil
		}
		if c.Path == "" {
			return nil, fmt.Errorf("missing path for SQLITE source")
		}
		sq := sqlite.New(
			filepath.Join(env.DataDir, c.Path),
			env.Migrations,
		)
		if env.Databases == nil {
			env.Databases = make(map[string]*sqlite.Source)
		}
		env.Databases[c.Path] = sq
		s = sq

	case SourceTypeGoexif:
		s = goexif.Exif{
			Width:  c.Width,
			Height: c.Height,
		}

	case SourceTypeThumb:
		s = thumb.New(
			c.Name,
			c.Path,
			c.Fit,
			c.Width,
			c.Height,
		)

	case SourceTypeImage:
		s = goimage.Image{
			Width:  c.Width,
			Height: c.Height,
		}

	case SourceTypeFFmpeg:
		s = ffmpeg.FFmpeg{
			Path:   env.FFmpegPath,
			Width:  c.Width,
			Height: c.Height,
			Fit:    c.Fit,
		}

	case SourceTypeDjpeg:
		d := djpeg.Djpeg{
			Path:   env.DjpegPath,
			Width:  c.Width,
			Height: c.Height,
			Scale:  c.Scale,
		}
		if c.Scale != "" {
			parts := strings.Split(c.Scale, "/")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid scale format: %s", c.Scale)
			}
			m, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid scale numerator: %s", parts[0])
			}
			n, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid scale denominator: %s", parts[1])
			}
			if n != 8 {
				return nil, fmt.Errorf("invalid scale denominator: %d, must be 8", n)
			}
			d.ScaleM = m
			d.ScaleN = n
		}
		s = d

	default:
		return nil, fmt.Errorf("unknown source type: %s", c.Type)
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

	s = configured.New(
		c.Name,
		c.Cost,
		s,
	)

	// println(s.Name(), c.Cost.Time.String(), c.Cost.TimePerOriginalMegapixel.String(), c.Cost.TimePerResizedMegapixel.String())

	return s, nil
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
		sources = append(sources, s)
	}
	return sources, nil
}
