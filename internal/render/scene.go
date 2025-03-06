package render

import (
	"context"
	"image/color"
	"math"
	"runtime"
	"runtime/trace"
	"sync"
	"time"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/draw"

	"photofield/internal/clip"
	"photofield/internal/image"
	"photofield/io"
)

type QualityPreset int

const (
	QualityPresetFast QualityPreset = iota
	QualityPresetHigh
)

type Render struct {
	TileSize          int         `json:"tile_size"`
	MaxSolidPixelArea float64     `json:"max_solid_pixel_area"`
	BackgroundColor   color.Color `json:"background_color"`
	Color             color.Color `json:"color"`
	TransparencyMask  bool        `json:"transparency_mask"`
	LogDraws          bool

	Sources io.Sources

	Selected image.Ids

	DebugOverdraw   bool
	DebugThumbnails bool
	QualityPreset   QualityPreset

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
	GetRegionClosestTo(Point, *Scene, RegionConfig) (region Region, ok bool)
}

type SceneId = string

type Dependency interface {
	UpdatedAt() time.Time
}

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
	Stale           bool           `json:"stale"`
	Dependencies    []Dependency   `json:"-"`
}

func (scene *Scene) UpdateStaleness() {
	for _, dep := range scene.Dependencies {
		if dep.UpdatedAt().After(scene.CreatedAt) {
			scene.Stale = true
			return
		}
	}
	scene.Stale = false
}

type Scales struct {
	Pixel float64
	Tile  float64
}

type PhotoRef struct {
	Index int
	Photo *Photo
}

func drawPhotoRefs(ctx context.Context, id int, photoRefs <-chan PhotoRef, config *Render, scene *Scene, c *canvas.Context, scales Scales, wg *sync.WaitGroup, source *image.Source) {
	trace.WithRegion(ctx, "drawPhotoRefs", func() {
		for photoRef := range photoRefs {
			selected := config.Selected.Contains(int(photoRef.Photo.Id))
			photoRef.Photo.Draw(ctx, config, scene, c, scales, source, selected)
		}
		wg.Done()
	})
}

// Workaround for "determinant of affine transformation matrix is zero"
// error when inverting the matrix using m.Inv() for very large canvases.
func invertMatrix(m canvas.Matrix) canvas.Matrix {
	det := m.Det()
	if det == 0.0 {
		panic("matrix is not invertible: determinant is zero")
	}
	return canvas.Matrix{{
		m[1][1] / det,
		-m[0][1] / det,
		-(m[1][1]*m[0][2] - m[0][1]*m[1][2]) / det,
	}, {
		-m[1][0] / det,
		m[0][0] / det,
		-(-m[1][0]*m[0][2] + m[0][0]*m[1][2]) / det,
	}}
}

func (scene *Scene) Draw(ctx context.Context, config *Render, c *canvas.Context, scales Scales, source *image.Source) {
	trace.WithRegion(ctx, "solid.Draw", func() {
		for i := range scene.Solids {
			solid := &scene.Solids[i]
			solid.Draw(c, scales)
		}
	})

	// for i := range scene.Photos {
	// 	photo := &scene.Photos[i]
	// 	photo.Draw(config, scene, c, scales, source)
	// }

	concurrent := runtime.NumCPU()
	photoCount := len(scene.Photos)
	if photoCount < concurrent {
		concurrent = photoCount
	}

	// startTime := time.Now()

	tileRect := Rect{X: 0, Y: 0, W: (float64)(config.TileSize), H: (float64)(config.TileSize)}

	tileToCanvas := invertMatrix(c.View())
	tileCanvasRect := tileRect.Transform(tileToCanvas)
	tileCanvasRect.Y = -tileCanvasRect.Y - tileCanvasRect.H

	visiblePhotos := scene.GetVisiblePhotoRefs(ctx, tileCanvasRect, 0)

	wg := &sync.WaitGroup{}
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go drawPhotoRefs(ctx, i, visiblePhotos, config, scene, c, scales, wg, source)
	}
	wg.Wait()

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
	t := uint32(0)
	var photo Photo
	for y := 0; y < height; y++ {
		frac := (float64(y) + 0.5) / float64(height)
		for ; ty <= float64(y)+frac && i < len(scene.Photos); i++ {
			photo = scene.Photos[i]
			py := (photo.Sprite.Rect.Y + photo.Sprite.Rect.H) * scale
			// TODO: figure out why sometimes py can be NaN
			if !math.IsNaN(py) {
				ty = py
			}
		}
		info := photo.GetInfo(source)
		if !info.DateTime.IsZero() {
			_, timezoneOffsetSeconds := info.DateTime.Zone()
			t = uint32(info.DateTime.Unix() + int64(timezoneOffsetSeconds))
		}
		timestamps[y] = t
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

func (scene *Scene) GetVisiblePhotoRefs(ctx context.Context, view Rect, maxCount int) <-chan PhotoRef {
	defer trace.StartRegion(ctx, "GetVisiblePhotoRefs").End()
	out := make(chan PhotoRef, 10)

	go func() {
		defer trace.StartRegion(ctx, "GetVisiblePhotoRefs goroutine").End()
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

func (s *Scene) GetClosestPhotoRef(p Point) (ref PhotoRef, ok bool) {
	minIndex := -1
	minDistSq := math.MaxFloat64
	for i := range s.Photos {
		photo := &s.Photos[i]
		dx := photo.Sprite.Rect.X - p.X
		dy := photo.Sprite.Rect.Y - p.Y
		distSq := dx*dx + dy*dy
		if distSq < minDistSq {
			minDistSq = distSq
			minIndex = i
		}
	}
	if minIndex == -1 {
		return PhotoRef{}, false
	}
	return PhotoRef{
		Index: minIndex,
		Photo: &s.Photos[minIndex],
	}, true
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

func (scene *Scene) GetRegions(bounds Rect, limit int) []Region {
	query := RegionConfig{
		Limit: 100,
	}
	if limit > 0 {
		query.Limit = limit
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

func (scene *Scene) GetRegionsByImageId(id image.ImageId, limit int) []Region {
	query := RegionConfig{
		Limit: 100,
	}
	if limit > 0 {
		query.Limit = limit
	}
	if scene.RegionSource == nil {
		return []Region{}
	}
	return scene.RegionSource.GetRegionsFromImageId(id, scene, query)
}

func (scene *Scene) GetRegionClosestTo(p Point) (region Region, ok bool) {
	return scene.RegionSource.GetRegionClosestTo(p, scene, RegionConfig{})
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
