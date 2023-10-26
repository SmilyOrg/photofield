package render

import (
	"image/color"
	"math"
	"sync"
	"time"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/draw"

	"photofield/internal/clip"
	"photofield/internal/image"
	"photofield/io"
)

type Render struct {
	TileSize          int         `json:"tile_size"`
	MaxSolidPixelArea float64     `json:"max_solid_pixel_area"`
	BackgroundColor   color.Color `json:"background_color"`
	TransparencyMask  bool        `json:"transparency_mask"`
	LogDraws          bool

	Sources io.Sources

	Selected image.Ids

	DebugOverdraw   bool
	DebugThumbnails bool

	Zoom        int
	CanvasImage draw.Image
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (p Point) Distance(other Point) float64 {
	dx := p.X - other.X
	dy := p.Y - other.Y
	return math.Sqrt(dx*dx + dy*dy)
}

type Region struct {
	Id     int         `json:"id"`
	Bounds Rect        `json:"bounds"`
	Data   interface{} `json:"data"`
}

type RegionConfig struct {
	Limit int
}

type Fonts struct {
	Main   canvas.FontFamily
	Header canvas.FontFace
	Hour   canvas.FontFace
	Debug  canvas.FontFace
}

type RegionSource interface {
	GetRegionsFromBounds(Rect, *Scene, RegionConfig) []Region
	GetRegionsFromImageId(image.ImageId, *Scene, RegionConfig) []Region
	GetRegionChanFromBounds(Rect, *Scene, RegionConfig) <-chan Region
	GetRegionById(int, *Scene, RegionConfig) Region
}

type SceneId = string

type Scene struct {
	Id              SceneId        `json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	Search          string         `json:"search,omitempty"`
	SearchEmbedding clip.Embedding `json:"-"`
	Loading         bool           `json:"loading"`
	LoadCount       int            `json:"load_count,omitempty"`
	LoadUnit        string         `json:"load_unit,omitempty"`
	Error           string         `json:"error,omitempty"`
	Fonts           Fonts          `json:"-"`
	Bounds          Rect           `json:"bounds"`
	Photos          []Photo        `json:"-"`
	FileCount       int            `json:"file_count"`
	Solids          []Solid        `json:"-"`
	Texts           []Text         `json:"-"`
	RegionSource    RegionSource   `json:"-"`
}

type Scales struct {
	Pixel float64
	Tile  float64
}

type PhotoRef struct {
	Index int
	Photo *Photo
}

func drawPhotoRefs(id int, photoRefs <-chan PhotoRef, counts chan int, config *Render, scene *Scene, c *canvas.Context, scales Scales, wg *sync.WaitGroup, source *image.Source) {
	count := 0
	for photoRef := range photoRefs {
		selected := config.Selected.Contains(int(photoRef.Photo.Id))
		photoRef.Photo.Draw(config, scene, c, scales, source, selected)
		count++
	}
	wg.Done()
	counts <- count
}

func (scene *Scene) Draw(config *Render, c *canvas.Context, scales Scales, source *image.Source) {
	for i := range scene.Solids {
		solid := &scene.Solids[i]
		solid.Draw(c, scales)
	}

	// for i := range scene.Photos {
	// 	photo := &scene.Photos[i]
	// 	photo.Draw(config, scene, c, scales, source)
	// }

	concurrent := 10
	photoCount := len(scene.Photos)
	if photoCount < concurrent {
		concurrent = photoCount
	}

	// startTime := time.Now()

	tileRect := Rect{X: 0, Y: 0, W: (float64)(config.TileSize), H: (float64)(config.TileSize)}
	tileToCanvas := c.View().Inv()
	tileCanvasRect := tileRect.Transform(tileToCanvas)
	tileCanvasRect.Y = -tileCanvasRect.Y - tileCanvasRect.H

	visiblePhotos := scene.GetVisiblePhotoRefs(tileCanvasRect, 0)
	visiblePhotoCount := 0

	wg := &sync.WaitGroup{}
	wg.Add(concurrent)
	counts := make(chan int)
	for i := 0; i < concurrent; i++ {
		go drawPhotoRefs(i, visiblePhotos, counts, config, scene, c, scales, wg, source)
	}
	wg.Wait()
	for i := 0; i < concurrent; i++ {
		visiblePhotoCount += <-counts
	}

	// micros := time.Since(startTime).Microseconds()
	// log.Printf("scene draw %5d / %5d photos, %6d μs all, %.2f μs / photo\n", visiblePhotoCount, photoCount, micros, float64(micros)/float64(visiblePhotoCount))

	for i := range scene.Texts {
		text := &scene.Texts[i]
		text.Draw(config, c, scales)
	}
}

func (scene *Scene) GetTimestamps(height int, source *image.Source) []uint32 {
	scale := float64(height) / scene.Bounds.H
	timestamps := make([]uint32, height)

	i := 0
	ty := -1.
	var t time.Time
	for y := 0; y < height; y++ {
		for ; ty <= float64(y) && i < len(scene.Photos); i++ {
			photo := scene.Photos[i]
			info := photo.GetInfo(source)
			t = info.DateTime
			py := (photo.Sprite.Rect.Y + photo.Sprite.Rect.H) * scale
			// TODO: figure out why sometimes py can be NaN
			if !math.IsNaN(py) {
				ty = py
			}
		}
		timestamps[y] = uint32(t.Unix())
	}

	return timestamps
}

func (scene *Scene) AddPhotosFromIds(ids <-chan image.ImageId) {
	for id := range ids {
		photo := Photo{}
		photo.Id = id
		scene.Photos = append(scene.Photos, photo)
	}
	scene.FileCount = len(scene.Photos)
}

func (scene *Scene) AddPhotosFromIdSlice(ids []image.ImageId) {
	for _, id := range ids {
		photo := Photo{}
		photo.Id = id
		scene.Photos = append(scene.Photos, photo)
	}
	scene.FileCount = len(scene.Photos)
}

func (scene *Scene) GetVisiblePhotoRefs(view Rect, maxCount int) <-chan PhotoRef {
	out := make(chan PhotoRef)
	go func() {
		count := 0
		if maxCount == 0 {
			maxCount = len(scene.Photos)
		}
		for i := range scene.Photos {
			photo := &scene.Photos[i]
			if photo.Sprite.Rect.IsVisible(view) {
				out <- PhotoRef{
					Index: i,
					Photo: photo,
				}
				count++
				if count >= maxCount {
					break
				}
			}
		}
		close(out)
	}()
	return out
}

func (scene *Scene) GetVisiblePhotos(view Rect) <-chan Photo {
	out := make(chan Photo, 100)
	go func() {
		for i := range scene.Photos {
			photo := &scene.Photos[i]
			if photo.Sprite.Rect.IsVisible(view) {
				out <- *photo
			}
		}
		close(out)
	}()
	return out
}

type BitmapAtZoom struct {
	Bitmap   Bitmap
	ZoomDist float64
}

func (scene *Scene) GetRegions(bounds Rect, limit *int) []Region {
	query := RegionConfig{
		Limit: 100,
	}
	if limit != nil {
		query.Limit = *limit
	}
	if scene.RegionSource == nil {
		return []Region{}
	}
	return scene.RegionSource.GetRegionsFromBounds(
		bounds,
		scene,
		query,
	)
}

func (scene *Scene) GetRegionsByImageId(id image.ImageId, limit *int) []Region {
	query := RegionConfig{
		Limit: 100,
	}
	if limit != nil {
		query.Limit = *limit
	}
	if scene.RegionSource == nil {
		return []Region{}
	}
	return scene.RegionSource.GetRegionsFromImageId(id, scene, query)
}

func (scene *Scene) GetRegionChan(bounds Rect) <-chan Region {
	if scene.RegionSource == nil {
		return nil
	}
	return scene.RegionSource.GetRegionChanFromBounds(
		bounds,
		scene,
		RegionConfig{},
	)
}

func (scene *Scene) GetRegion(id int) Region {
	if scene.RegionSource == nil {
		return Region{}
	}
	return scene.RegionSource.GetRegionById(id, scene, RegionConfig{})
}
