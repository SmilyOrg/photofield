package layout

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"photofield/internal/image"
	"photofield/internal/render"
	"photofield/io"
	"photofield/tag"
	"sort"
	"strings"
	"time"
)

type Type string

const (
	Album      Type = "ALBUM"
	Timeline   Type = "TIMELINE"
	Square     Type = "SQUARE"
	Wall       Type = "WALL"
	Map        Type = "MAP"
	Search     Type = "SEARCH"
	Strip      Type = "STRIP"
	Highlights Type = "HIGHLIGHTS"
	Flex       Type = "FLEX"
)

type Order int

const (
	None     Order = iota
	DateAsc  Order = iota
	DateDesc Order = iota
)

func OrderFromSort(s string) Order {
	switch s {
	case "+date":
		return DateAsc
	case "-date":
		return DateDesc
	default:
		return None
	}
}

type Layout struct {
	Type           Type  `json:"type"`
	Order          Order `json:"order"`
	ViewportWidth  float64
	ViewportHeight float64
	ImageHeight    float64
	ImageSpacing   float64
	LineSpacing    float64
}

type Section struct {
	infos    []image.SourcedInfo
	Inverted bool
}

type SectionPhoto struct {
	render.Photo
	Size image.Size
}

type Photo struct {
	Index int
	Photo render.Photo
	Info  image.Info
}

type PhotoRegionSource struct {
	Source *image.Source
}

type RegionThumbnail struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Filename    string `json:"filename"`
}

type RegionTag struct {
	Id string `json:"id"`
}

type PhotoRegionData struct {
	Id         int               `json:"id"`
	Path       string            `json:"path"`
	Filename   string            `json:"filename"`
	Extension  string            `json:"extension"`
	Video      bool              `json:"video"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	CreatedAt  string            `json:"created_at"`
	Thumbnails []RegionThumbnail `json:"thumbnails"`
	Tags       []tag.Tag         `json:"tags"`
	// SmallestThumbnail     string   `json:"smallest_thumbnail"`
}

func (regionSource PhotoRegionSource) getRegionFromPhoto(id int, photo *render.Photo, scene *render.Scene, regionConfig render.RegionConfig) render.Region {

	source := regionSource.Source

	originalPath := photo.GetPath(source)
	info := source.GetInfo(photo.Id)

	originalSize := io.Size{
		X: info.Width,
		Y: info.Height,
	}
	isVideo := source.IsSupportedVideo(originalPath)
	extension := filepath.Ext(originalPath)
	filename := filepath.Base(originalPath)
	basename := strings.TrimSuffix(filename, extension)

	var thumbnails []RegionThumbnail

	for _, s := range source.Sources {
		if !s.Exists(context.TODO(), io.ImageId(id), originalPath) {
			continue
		}
		size := s.Size(originalSize)
		ext := s.Ext()
		if ext == "" {
			ext = extension
		}
		filename := fmt.Sprintf(
			"%s_%s%s",
			basename, s.Name(), ext,
		)
		thumbnails = append(thumbnails, RegionThumbnail{
			Name:        s.Name(),
			DisplayName: s.DisplayName(),
			Width:       size.X,
			Height:      size.Y,
			Filename:    filename,
		})
	}

	sort.Slice(thumbnails, func(i, j int) bool {
		a := &thumbnails[i]
		b := &thumbnails[j]
		aa := a.Width * a.Height
		bb := b.Width * b.Height
		if aa != bb {
			return aa < bb
		}
		return a.Name < b.Name
	})

	tags := make([]tag.Tag, 0)
	for tag := range source.ListImageTags(photo.Id) {
		tags = append(tags, tag)
	}

	return render.Region{
		Id:     id,
		Bounds: photo.Sprite.Rect,
		Data: PhotoRegionData{
			Id:         int(photo.Id),
			Path:       originalPath,
			Filename:   filename,
			Extension:  extension,
			Video:      isVideo,
			Width:      info.Width,
			Height:     info.Height,
			CreatedAt:  info.DateTime.Format(time.RFC3339),
			Thumbnails: thumbnails,
			Tags:       tags,
		},
	}
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect render.Rect, scene *render.Scene, regionConfig render.RegionConfig) []render.Region {
	regions := make([]render.Region, 0)
	photos := scene.GetVisiblePhotoRefs(rect, regionConfig.Limit)
	for photo := range photos {
		regions = append(regions, regionSource.getRegionFromPhoto(
			1+photo.Index,
			photo.Photo,
			scene, regionConfig,
		))
	}
	return regions
}

func (regionSource PhotoRegionSource) GetRegionsFromImageId(id image.ImageId, scene *render.Scene, regionConfig render.RegionConfig) []render.Region {
	regions := make([]render.Region, 0)
	max := regionConfig.Limit
	if max == 0 {
		max = len(scene.Photos)
	}
	for i := range scene.Photos {
		photo := &scene.Photos[i]
		if photo.Id != id {
			continue
		}
		regions = append(regions, regionSource.getRegionFromPhoto(
			1+i,
			photo,
			scene, regionConfig,
		))
		if len(regions) >= max {
			break
		}
	}
	return regions
}

func (regionSource PhotoRegionSource) GetRegionChanFromBounds(rect render.Rect, scene *render.Scene, regionConfig render.RegionConfig) <-chan render.Region {
	out := make(chan render.Region)
	go func() {
		photos := scene.GetVisiblePhotoRefs(rect, regionConfig.Limit)
		for photo := range photos {
			out <- regionSource.getRegionFromPhoto(
				1+photo.Index,
				photo.Photo,
				scene, regionConfig,
			)
		}
		close(out)
	}()
	return out
}

func (regionSource PhotoRegionSource) GetRegionById(id int, scene *render.Scene, regionConfig render.RegionConfig) render.Region {
	if id <= 0 || id > len(scene.Photos) {
		return render.Region{}
	}
	photo := scene.Photos[id-1]
	return regionSource.getRegionFromPhoto(id, &photo, scene, regionConfig)
}

func layoutFitRow(row []SectionPhoto, bounds render.Rect, imageSpacing float64) float64 {
	count := len(row)
	if count == 0 {
		return 1.
	}
	firstPhoto := row[0]
	firstRect := firstPhoto.Photo.Sprite.Rect
	lastPhoto := row[count-1]
	lastRect := lastPhoto.Photo.Sprite.Rect
	totalSpacing := float64(count-1) * imageSpacing

	rowWidth := lastRect.X + lastRect.W
	scale := (bounds.W - totalSpacing) / (rowWidth - totalSpacing)
	x := firstRect.X
	for i := range row {
		photo := &row[i]
		rect := photo.Photo.Sprite.Rect
		photo.Photo.Sprite.Rect = render.Rect{
			X: x,
			Y: rect.Y,
			W: rect.W * scale,
			H: rect.H * scale,
		}
		x += photo.Photo.Sprite.Rect.W + imageSpacing
	}

	// fmt.Printf("fit row width %5.2f / %5.2f -> %5.2f  scale %.2f\n", rowWidth, bounds.W, lastPhoto.Photo.Original.Sprite.Rect.X+lastPhoto.Photo.Original.Sprite.Rect.W, scale)

	x -= imageSpacing
	return scale
}

func addSectionToScene(section *Section, scene *render.Scene, bounds render.Rect, config Layout, source *image.Source) render.Rect {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	i := 0

	row := make([]SectionPhoto, 0)

	for _, info := range section.infos {
		photo := SectionPhoto{
			Photo: render.Photo{
				Id:     info.Id,
				Sprite: render.Sprite{},
			},
			Size: image.Size{
				X: info.Width,
				Y: info.Height,
			},
		}

		aspectRatio := float64(photo.Size.X) / float64(photo.Size.Y)
		imageWidth := float64(config.ImageHeight) * aspectRatio

		if x+imageWidth > bounds.W {
			scale := layoutFitRow(row, bounds, config.ImageSpacing)
			for _, p := range row {
				scene.Photos = append(scene.Photos, p.Photo)
			}
			row = nil
			x = 0
			y += config.ImageHeight*scale + config.LineSpacing
		}

		photo.Photo.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			config.ImageHeight,
			float64(photo.Size.X),
			float64(photo.Size.Y),
		)

		// println(photo.GetPath(source), photo.Sprite.Rect.String(), bounds.X, bounds.Y, x, y, config.ImageHeight, photo.Size.X, photo.Size.Y)

		row = append(row, photo)

		x += imageWidth + config.ImageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout section %d\n", i)
		}
		i++
	}
	for _, p := range row {
		scene.Photos = append(scene.Photos, p.Photo)
	}
	x = 0
	y += config.ImageHeight + config.LineSpacing
	return render.Rect{
		X: bounds.X,
		Y: bounds.Y,
		W: bounds.W,
		H: y,
	}
}

func SameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
