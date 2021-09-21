package photofield

import (
	"log"
	"path/filepath"
	. "photofield/internal"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
	"strings"
	"time"
)

type LayoutType string

const (
	Album    LayoutType = "ALBUM"
	Timeline            = "TIMELINE"
	Square              = "SQUARE"
	Wall                = "WALL"
)

type Layout struct {
	Type         LayoutType `json:"type"`
	SceneWidth   float64
	ImageHeight  float64
	ImageSpacing float64
	LineSpacing  float64
}

type Section struct {
	infos    []SourcedImageInfo
	Inverted bool
}

type SectionPhoto struct {
	Index int
	Photo *Photo
	Size  Size
}

type LayoutPhoto struct {
	Index int
	Photo Photo
	Info  ImageInfo
}

type PhotoRegionSource struct {
	imageSource *storage.ImageSource
}

type RegionThumbnail struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
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

func (regionSource PhotoRegionSource) getRegionFromPhoto(id int, photo *Photo, scene *Scene, regionConfig RegionConfig) Region {

	source := regionSource.imageSource

	originalPath := photo.GetPath(source)
	info := source.GetImageInfo(originalPath)
	originalSize := Size{
		X: info.Width,
		Y: info.Height,
	}
	isVideo := source.IsSupportedVideo(originalPath)

	var thumbnailTemplates []Thumbnail
	if isVideo {
		thumbnailTemplates = source.Videos.Thumbnails
	} else {
		thumbnailTemplates = source.Images.Thumbnails
	}

	var thumbnails []RegionThumbnail
	for i := range thumbnailTemplates {
		thumbnail := &thumbnailTemplates[i]
		thumbnailPath := thumbnail.GetPath(originalPath)
		if source.Exists(thumbnailPath) {
			thumbnailSize := thumbnail.Fit(originalSize)
			thumbnails = append(thumbnails, RegionThumbnail{
				Name:   thumbnail.Name,
				Width:  thumbnailSize.X,
				Height: thumbnailSize.Y,
			})
		}
	}

	return Region{
		Id:     id,
		Bounds: photo.Sprite.Rect,
		Data: PhotoRegionData{
			Id:         int(photo.Id),
			Path:       originalPath,
			Filename:   filepath.Base(originalPath),
			Extension:  strings.ToLower(filepath.Ext(originalPath)),
			Video:      isVideo,
			Width:      info.Width,
			Height:     info.Height,
			CreatedAt:  info.DateTime.Format(time.RFC3339),
			Thumbnails: thumbnails,
		},
	}
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect Rect, scene *Scene, regionConfig RegionConfig) []Region {
	regions := make([]Region, 0)
	photos := scene.GetVisiblePhotos(rect, regionConfig.Limit)
	for photo := range photos {
		regions = append(regions, regionSource.getRegionFromPhoto(
			photo.Index,
			photo.Photo,
			scene, regionConfig,
		))
	}
	return regions
}

func (regionSource PhotoRegionSource) GetRegionById(id int, scene *Scene, regionConfig RegionConfig) Region {
	if id < 0 || id >= len(scene.Photos)-1 {
		return Region{Id: -1}
	}
	photo := scene.Photos[id]
	return regionSource.getRegionFromPhoto(id, &photo, scene, regionConfig)
}

func layoutFitRow(row []SectionPhoto, bounds Rect, imageSpacing float64) float64 {
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
		photo := row[i]
		rect := photo.Photo.Sprite.Rect
		photo.Photo.Sprite.Rect = Rect{
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

func addSectionPhotos(section *Section, scene *Scene, source *storage.ImageSource) <-chan SectionPhoto {
	photos := make(chan SectionPhoto, 10000)
	go func() {
		startIndex := len(scene.Photos)
		for _, info := range section.infos {
			scene.Photos = append(scene.Photos, Photo{
				Id:     source.GetImageId(info.Path),
				Sprite: Sprite{},
			})
		}
		for index, info := range section.infos {
			sceneIndex := startIndex + index
			photo := &scene.Photos[sceneIndex]
			photos <- SectionPhoto{
				Index: sceneIndex,
				Photo: photo,
				Size: Size{
					X: info.Width,
					Y: info.Height,
				},
			}
		}
		close(photos)
	}()
	return photos
}

func layoutSectionPhotos(photos <-chan SectionPhoto, bounds Rect, config Layout, scene *Scene, source *storage.ImageSource) Rect {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	i := 0

	row := make([]SectionPhoto, 0)

	for photo := range photos {

		// log.Println("layout", photo.Index)

		aspectRatio := float64(photo.Size.X) / float64(photo.Size.Y)
		imageWidth := float64(config.ImageHeight) * aspectRatio

		if x+imageWidth > bounds.W {
			scale := layoutFitRow(row, bounds, config.ImageSpacing)
			row = nil
			x = 0
			y += config.ImageHeight*scale + config.LineSpacing
		}

		// fmt.Printf("%4.0f %4.0f %4.0f %4.0f %4.0f %4.0f %4.0f\n", bounds.X, bounds.Y, x, y, imageHeight, photo.Size.Width, photo.Size.Height)

		photo.Photo.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			config.ImageHeight,
			float64(photo.Size.X),
			float64(photo.Size.Y),
		)

		row = append(row, photo)

		// photoRect := photo.Photo.Original.Sprite.GetBounds()
		// scene.Regions = append(scene.Regions, Region{
		// 	Id: len(scene.Regions),
		// 	Bounds: Bounds{
		// 		X: photoRect.X,
		// 		Y: photoRect.Y,
		// 		W: photoRect.W,
		// 		H: photoRect.H,
		// 	},
		// })

		// fmt.Printf("%d %f %f %f\n", i, x, imageWidth, bounds.W)

		x += imageWidth + config.ImageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout section %d\n", photo.Index)
		}
		i++
	}
	x = 0
	y += config.ImageHeight + config.LineSpacing
	return Rect{
		X: bounds.X,
		Y: bounds.Y,
		W: bounds.W,
		H: y,
	}
}
