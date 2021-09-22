package render

import (
	"math"
	"sync"
	"time"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/draw"

	"photofield/internal/image"
)

type Render struct {
	TileSize          int     `json:"tile_size"`
	MaxSolidPixelArea float64 `json:"max_solid_pixel_area"`
	LogDraws          bool
	DebugOverdraw     bool
	DebugThumbnails   bool

	Zoom        int
	CanvasImage draw.Image
}

type Transform struct {
	view canvas.Matrix
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
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
	GetRegionById(int, *Scene, RegionConfig) Region
}

type SceneId = string

type Scene struct {
	Id           SceneId      `json:"id"`
	CreatedAt    time.Time    `json:"created_at"`
	Fonts        Fonts        `json:"-"`
	Bounds       Rect         `json:"bounds"`
	Photos       []Photo      `json:"-"`
	FileCount    int          `json:"file_count"`
	Solids       []Solid      `json:"-"`
	Texts        []Text       `json:"-"`
	RegionSource RegionSource `json:"-"`
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
		photoRef.Photo.Draw(config, scene, c, scales, source)
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

	visiblePhotos := scene.GetVisiblePhotos(tileCanvasRect, math.MaxInt32)
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
		text.Draw(c, scales)
	}
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

func (scene *Scene) GetVisiblePhotos(view Rect, maxCount int) <-chan PhotoRef {
	out := make(chan PhotoRef)
	go func() {
		count := 0
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

type BitmapAtZoom struct {
	Bitmap   Bitmap
	ZoomDist float64
}

func (scene *Scene) getRegionScale() float64 {
	return scene.Bounds.W
}

func (scene *Scene) GetRegions(config *Render, bounds Rect, limit *int) []Region {
	query := RegionConfig{
		Limit: 100,
	}
	if limit != nil {
		query.Limit = *limit
	}
	return scene.RegionSource.GetRegionsFromBounds(
		bounds,
		scene,
		query,
	)
}

func (scene *Scene) GetRegion(id int) Region {
	return scene.RegionSource.GetRegionById(id, scene, RegionConfig{})
}
