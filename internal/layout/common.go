package layout

import (
	"log"
	"path/filepath"
	"photofield/internal/image"
	"photofield/internal/render"
	"strings"
	"time"
)

type Type string

const (
	Album    Type = "ALBUM"
	Timeline Type = "TIMELINE"
	Square   Type = "SQUARE"
	Wall     Type = "WALL"
	Search   Type = "SEARCH"
	Strip    Type = "STRIP"
)

type Layout struct {
	Type           Type `json:"type"`
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
	Name     string `json:"name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Filename string `json:"filename"`
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
	// SmallestThumbnail     string   `json:"smallest_thumbnail"`
}

func (regionSource PhotoRegionSource) getRegionFromPhoto(id int, photo *render.Photo, scene *render.Scene, regionConfig render.RegionConfig) render.Region {

	source := regionSource.Source

	originalPath := photo.GetPath(source)
	info := source.GetInfo(photo.Id)
	// originalSize := image.Size{
	// 	X: info.Width,
	// 	Y: info.Height,
	// }
	isVideo := source.IsSupportedVideo(originalPath)
	extension := strings.ToLower(filepath.Ext(originalPath))
	filename := filepath.Base(originalPath)

	// thumbnailTemplates := source.GetApplicableThumbnails(originalPath)
	var thumbnails []RegionThumbnail
	// for i := range thumbnailTemplates {
	// 	thumbnail := &thumbnailTemplates[i]
	// 	thumbnailPath := thumbnail.GetPath(originalPath)
	// 	if source.Exists(thumbnailPath) {
	// 		thumbnailSize := thumbnail.Fit(originalSize)
	// 		basename := strings.TrimSuffix(filename, extension)
	// 		thumbnailFilename := fmt.Sprintf(
	// 			"%s_%s%s",
	// 			basename, thumbnail.Name, filepath.Ext(thumbnailPath),
	// 		)
	// 		thumbnails = append(thumbnails, RegionThumbnail{
	// 			Name:     thumbnail.Name,
	// 			Width:    thumbnailSize.X,
	// 			Height:   thumbnailSize.Y,
	// 			Filename: thumbnailFilename,
	// 		})
	// 		println("t", thumbnailSize.X)
	// 	}
	// }

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
		},
	}
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect render.Rect, scene *render.Scene, regionConfig render.RegionConfig) []render.Region {
	regions := make([]render.Region, 0)
	photos := scene.GetVisiblePhotos(rect, regionConfig.Limit)
	for photo := range photos {
		regions = append(regions, regionSource.getRegionFromPhoto(
			1+photo.Index,
			photo.Photo,
			scene, regionConfig,
		))
	}
	return regions
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
