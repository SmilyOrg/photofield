package photofield

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	. "photofield/internal"
	storage "photofield/internal/storage"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

type RenderConfig struct {
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

type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Sprite struct {
	Rect Rect
}
type Bitmap struct {
	Path   string
	Sprite Sprite
}
type Solid struct {
	Sprite Sprite
	Color  color.Color
}
type Text struct {
	Sprite Sprite
	Font   *canvas.FontFace
	Text   string
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
	Header canvas.FontFace
	Hour   canvas.FontFace
	Debug  canvas.FontFace
}

type RegionSource interface {
	GetRegionsFromBounds(Rect, *Scene, RegionConfig) []Region
	GetRegionById(int, *Scene, RegionConfig) Region
}

type Scene struct {
	Fonts        Fonts        `json:"-"`
	Bounds       Rect         `json:"bounds"`
	Photos       []Photo      `json:"-"`
	PhotoCount   int          `json:"photoCount"`
	Solids       []Solid      `json:"-"`
	Texts        []Text       `json:"-"`
	RegionSource RegionSource `json:"-"`
}

type Scales struct {
	Pixel float64
	Tile  float64
}

type Photo struct {
	Id     storage.ImageId
	Sprite Sprite
}

type PhotoRef struct {
	Index int
	Photo *Photo
}

func NewRectFromCanvasRect(r canvas.Rect) Rect {
	return Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}
}

func (rect Rect) ToCanvasRect() canvas.Rect {
	return canvas.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: rect.H}
}

func (rect Rect) Move(offset Point) Rect {
	rect.X += offset.X
	rect.Y += offset.Y
	return rect
}

func (rect Rect) ScalePoint(scale Point) Rect {
	rect.X *= scale.X
	rect.W *= scale.X
	rect.Y *= scale.Y
	rect.H *= scale.Y
	return rect
}

func (rect Rect) Scale(scale float64) Rect {
	rect.X *= scale
	rect.W *= scale
	rect.Y *= scale
	rect.H *= scale
	return rect
}

func (rect Rect) Transform(m canvas.Matrix) Rect {
	return NewRectFromCanvasRect(rect.ToCanvasRect().Transform(m))
}

func (rect Rect) String() string {
	return fmt.Sprintf("%3.3f %3.3f %3.3f %3.3f", rect.X, rect.Y, rect.W, rect.H)
}

func (rect Rect) FitInside(container Rect) Rect {
	imageRatio := rect.W / rect.H

	var scale float64
	if container.W/container.H < imageRatio {
		scale = container.W / rect.W
	} else {
		scale = container.H / rect.H
	}

	return Rect{
		X: container.X,
		Y: container.Y,
		W: rect.W * scale,
		H: rect.H * scale,
	}
}

func NewSolidFromRect(rect Rect, color color.Color) Solid {
	solid := Solid{}
	solid.Color = color
	solid.Sprite.Rect = rect
	return solid
}

func NewTextFromRect(rect Rect, font *canvas.FontFace, txt string) Text {
	text := Text{}
	text.Text = txt
	text.Font = font
	text.Sprite.Rect = rect
	return text
}

func drawPhotosSlice(photos []Photo, config *RenderConfig, scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) {
	for i := range photos {
		photo := &photos[i]
		photo.Draw(config, scene, c, scales, source)
	}
}

func drawPhotoChannel(id int, index chan int, config *RenderConfig, scene *Scene, c *canvas.Context, scales Scales, wg *sync.WaitGroup, source *storage.ImageSource) {
	for i := range index {
		photo := &scene.Photos[i]
		photo.Draw(config, scene, c, scales, source)
	}
	wg.Done()
}

func (scene *Scene) Draw(config *RenderConfig, c *canvas.Context, scales Scales, source *storage.ImageSource) {
	for i := range scene.Solids {
		solid := &scene.Solids[i]
		solid.Draw(c, scales)
	}

	// for i := range scene.Photos {
	// 	photo := &scene.Photos[i]
	// 	photo.Draw(config, scene, c, scales, source)
	// }

	index := make(chan int)

	concurrent := 10
	photoCount := len(scene.Photos)
	if photoCount < concurrent {
		concurrent = photoCount
	}

	wg := &sync.WaitGroup{}
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go drawPhotoChannel(i, index, config, scene, c, scales, wg, source)
	}

	var lastLogTime time.Time
	var logInterval time.Duration
	lastLogIndex := 0
	if config.LogDraws {
		lastLogTime = time.Now()
		logInterval = 1 * time.Second
	}
	for i := range scene.Photos {
		index <- i
		if config.LogDraws {
			now := time.Now()
			elapsed := now.Sub(lastLogTime)
			if elapsed > logInterval {
				perSec := float64(i-lastLogIndex) / elapsed.Seconds()
				log.Printf("draw photo %d, %.2f / sec \n", i, perSec)
				lastLogTime = now
				lastLogIndex = i
			}
		}
	}
	close(index)
	wg.Wait()

	for i := range scene.Texts {
		text := &scene.Texts[i]
		text.Draw(c, scales)
	}
}

func (scene *Scene) AddPhotosFromIds(ids <-chan storage.ImageId) {
	for id := range ids {
		photo := Photo{}
		photo.Id = id
		scene.Photos = append(scene.Photos, photo)
	}
	scene.PhotoCount = len(scene.Photos)
}

func (scene *Scene) AddPhotosFromIdSlice(ids []storage.ImageId) {
	for _, id := range ids {
		photo := Photo{}
		photo.Id = id
		scene.Photos = append(scene.Photos, photo)
	}
	scene.PhotoCount = len(scene.Photos)
}

func (scene *Scene) GetVisiblePhotos(output chan PhotoRef, view Rect, maxCount int) {
	count := 0
	for i := range scene.Photos {
		photo := &scene.Photos[i]
		if photo.Sprite.Rect.IsVisible(view) {
			output <- PhotoRef{
				Index: i,
				Photo: photo,
			}
			count++
			if count >= maxCount {
				break
			}
		}
	}
	close(output)
}

func RenderImageFast(rimg draw.Image, img image.Image, m canvas.Matrix) {
	origin := m.Dot(canvas.Point{X: 0, Y: float64(img.Bounds().Size().Y)})
	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{m[0][0], -m[0][1], origin.X, -m[1][0], m[1][1], h - origin.Y}
	draw.ApproxBiLinear.Transform(rimg, aff3, img, img.Bounds(), draw.Src, nil)
}

func (rect *Rect) GetMatrix() canvas.Matrix {
	return canvas.Identity.
		Translate(rect.X, -rect.Y-rect.H)
}

func (rect *Rect) GetMatrixFitWidth(width float64) canvas.Matrix {
	scale := rect.W / width
	return rect.GetMatrix().
		Scale(scale, scale)
}

func (rect *Rect) GetMatrixFitImage(image *image.Image) canvas.Matrix {
	bounds := (*image).Bounds()
	return rect.GetMatrixFitWidth(float64(bounds.Max.X) - float64(bounds.Min.X))
}

func (rect *Rect) GetMatrixFitImageRotate(image *image.Image) canvas.Matrix {
	bounds := (*image).Bounds()
	rectAspectRatio := rect.W / rect.H
	imageWidth := float64(bounds.Max.X - bounds.Min.X)
	imageHeight := float64(bounds.Max.Y - bounds.Min.Y)
	imageAspectRatio := imageWidth / imageHeight
	imageAspectRatioRotated := 1 / imageAspectRatio
	var matrix canvas.Matrix
	if math.Abs(rectAspectRatio-imageAspectRatio) < math.Abs(rectAspectRatio-imageAspectRatioRotated) {
		matrix = rect.GetMatrixFitWidth(imageWidth)
	} else {
		imageWidth, imageHeight = imageHeight, imageWidth
		matrix = rect.GetMatrixFitWidth(imageWidth).Translate(0, imageHeight).Rotate(-90)
	}
	return matrix
}

func (bitmap *Bitmap) Draw(rimg draw.Image, c *canvas.Context, scales Scales, source *storage.ImageSource) error {
	if bitmap.Sprite.IsVisible(c, scales) {
		image, err := source.GetImage(bitmap.Path)
		if err != nil {
			return err
		}

		model := bitmap.Sprite.Rect.GetMatrixFitImageRotate(image)
		m := c.View().Mul(model)

		RenderImageFast(rimg, *image, m)
	}
	return nil
}

func (bitmap *Bitmap) GetSize(source *storage.ImageSource) Size {
	info := source.GetImageInfo(bitmap.Path)
	return Size{X: info.Width, Y: info.Height}
}

func (photo *Photo) GetSize(source *storage.ImageSource) Size {
	info := source.GetImageInfo(source.GetImagePath(photo.Id))
	return Size{X: info.Width, Y: info.Height}
}

func (sprite *Sprite) PlaceFitHeight(
	x float64,
	y float64,
	fitHeight float64,
	contentWidth float64,
	contentHeight float64,
) {
	scale := fitHeight / contentHeight

	sprite.Rect = Rect{
		X: x,
		Y: y,
		W: contentWidth * scale,
		H: contentHeight * scale,
	}
}

func (sprite *Sprite) PlaceFit(
	x float64,
	y float64,
	fitWidth float64,
	fitHeight float64,
	contentWidth float64,
	contentHeight float64,
) {
	imageRatio := contentWidth / contentHeight

	var scale float64
	if fitWidth/fitHeight < imageRatio {
		scale = fitWidth / contentWidth
		// y = y - fitHeight*0.5 + scale*contentHeight*0.5
	} else {
		scale = fitHeight / contentHeight
		// x = x - width*0.5 + scale*contentWidth*0.5
	}

	sprite.Rect = Rect{
		X: x,
		Y: y,
		W: contentWidth * scale,
		H: contentHeight * scale,
	}
}

func (photo *Photo) Place(x float64, y float64, width float64, height float64, source *storage.ImageSource) {
	imageSize := photo.GetSize(source)
	imageWidth := float64(imageSize.X)
	imageHeight := float64(imageSize.Y)

	photo.Sprite.PlaceFit(x, y, width, height, imageWidth, imageHeight)
}

func (sprite *Sprite) Draw(c *canvas.Context) {
	c.RenderPath(
		canvas.Rectangle(sprite.Rect.W, sprite.Rect.H),
		c.Style,
		c.View().Mul(sprite.Rect.GetMatrix()),
	)
}

func (sprite *Sprite) DrawWithStyle(c *canvas.Context, style canvas.Style) {
	c.RenderPath(
		canvas.Rectangle(sprite.Rect.W, sprite.Rect.H),
		style,
		c.View().Mul(sprite.Rect.GetMatrix()),
	)
}

func (text *Text) Draw(c *canvas.Context, scales Scales) {
	if text.Sprite.IsVisible(c, scales) {
		textLine := canvas.NewTextLine(*text.Font, text.Text, canvas.Left)
		c.RenderText(textLine, c.View().Mul(text.Sprite.Rect.GetMatrix()))
	}
}

func getRGBA(col color.Color) color.RGBA {
	r, g, b, a := col.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func (bitmap *Bitmap) DrawOverdraw(c *canvas.Context, source *storage.ImageSource) {
	style := c.Style

	size := bitmap.GetSize(source)
	pixelZoom := bitmap.Sprite.Rect.GetPixelZoom(c, size)
	barWidth := -pixelZoom * 0.1
	// barHeight := 0.04
	alpha := pixelZoom * 0.1 * 0xFF
	max := 0.8 * float64(0xFF)
	if barWidth > 0 {
		alpha = math.Min(max, math.Max(0, -alpha))
		style.FillColor = getRGBA(color.NRGBA{0xFF, 0x00, 0x00, uint8(alpha)})
	} else {
		alpha = math.Min(max, math.Max(0, alpha))
		style.FillColor = getRGBA(color.NRGBA{0x00, 0x00, 0xFF, uint8(alpha)})
	}

	bitmap.Sprite.DrawWithStyle(c, style)

	// style.FillColor = canvas.Yellowgreen
	// c.RenderPath(
	// 	canvas.Rectangle(bitmap.Sprite.Rect.W*0.5*barWidth, bitmap.Sprite.Rect.H*barHeight),
	// 	style,
	// 	c.View().Mul(bitmap.Sprite.Rect.GetMatrix()).
	// 		Translate(
	// 			bitmap.Sprite.Rect.W*0.5,
	// 			bitmap.Sprite.Rect.H*(0.5-barHeight*0.5),
	// 		),
	// )
}

func (sprite *Sprite) DrawText(c *canvas.Context, scales Scales, font *canvas.FontFace, txt string) {
	text := NewTextFromRect(sprite.Rect, font, txt)
	text.Draw(c, scales)
}

func (sprite *Sprite) IsVisible(c *canvas.Context, scales Scales) bool {
	rect := canvas.Rect{X: 0, Y: 0, W: sprite.Rect.W, H: sprite.Rect.H}
	canvasToUnit := canvas.Identity.
		Scale(scales.Tile, scales.Tile).
		Mul(c.View().Mul(sprite.Rect.GetMatrix()))
	unitRect := rect.Transform(canvasToUnit)
	return unitRect.X <= 1 && unitRect.Y <= 1 && unitRect.X+unitRect.W >= 0 && unitRect.Y+unitRect.H >= 0
}

func (rect *Rect) IsVisible(view Rect) bool {
	return rect.X <= view.X+view.W &&
		rect.Y <= view.Y+view.H &&
		rect.X+rect.W >= view.X &&
		rect.Y+rect.H >= view.Y
}

func (rect *Rect) GetPixelArea(c *canvas.Context, size Size) float64 {
	pixel := canvas.Rect{X: 0, Y: 0, W: 1, H: 1}
	canvasToTile := c.View().Mul(rect.GetMatrixFitWidth(float64(size.X)))
	tileRect := pixel.Transform(canvasToTile)
	// fmt.Printf("rect w %4.0f h %4.0f   size w %4.0f h %4.0f   tileRect w %4f h %4f\n", rect.W, rect.H, size.Width, size.Height, tileRect.W, tileRect.H)
	// tx, ty, theta, sx, sy, phi := canvasToTile.Decompose()
	// log.Printf("tx %f ty %f theta %f sx %f sy %f phi %f rectw %f tw %f th %f\n", tx, ty, theta, sx, sy, phi, rect.W, tileRect.W, tileRect.H)
	area := tileRect.W * tileRect.H
	return area
}

func (rect *Rect) GetPixelZoom(c *canvas.Context, size Size) float64 {
	pixelArea := rect.GetPixelArea(c, size)
	if pixelArea >= 1 {
		return pixelArea
	} else {
		return -1 / pixelArea
	}
}

func (rect *Rect) GetPixelZoomDist(c *canvas.Context, size Size) float64 {
	return math.Abs(rect.GetPixelZoom(c, size))
}

func (photo *Photo) getBestBitmap(config *RenderConfig, scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) (Bitmap, float64) {
	var best *Thumbnail
	originalSize := photo.GetSize(source)
	originalPath := source.GetImagePath(photo.Id)
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}
	// fmt.Printf("%4.0f %4.0f\n", photo.Original.Sprite.Rect.W, photo.Original.Sprite.Rect.H)
	bestZoomDist := originalZoomDist
	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
		thumbSize := thumbnail.Fit(originalSize)
		zoomDist := photo.Sprite.Rect.GetPixelZoomDist(c, thumbSize)
		if zoomDist < bestZoomDist {
			thumbnailPath := thumbnail.GetPath(originalPath)
			if source.Exists(thumbnailPath) {
				best = thumbnail
				bestZoomDist = zoomDist
			}
		}
		// fmt.Printf("orig w %4.0f h %4.0f   thumb w %4.0f h %4.0f   zoom dist best %8.2f cur %8.2f area %8.6f\n", originalSize.Width, originalSize.Height, thumbSize.Width, thumbSize.Height, bestZoomDist, zoomDist, photo.Original.Sprite.Rect.GetPixelArea(c, thumbSize))
	}

	if best == nil {
		return Bitmap{
			Path:   originalPath,
			Sprite: photo.Sprite,
		}, originalZoomDist
	}

	return Bitmap{
		Path: best.GetPath(originalPath),
		Sprite: Sprite{
			Rect: photo.Sprite.Rect,
		},
	}, bestZoomDist
}

type BitmapAtZoom struct {
	Bitmap   Bitmap
	ZoomDist float64
}

func (photo *Photo) getBestBitmaps(config *RenderConfig, scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) []BitmapAtZoom {

	originalSize := photo.GetSize(source)
	originalPath := source.GetImagePath(photo.Id)
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}

	bitmaps := make([]BitmapAtZoom, 1+len(source.Thumbnails))
	bitmaps[0] = BitmapAtZoom{
		Bitmap: Bitmap{
			Path:   originalPath,
			Sprite: photo.Sprite,
		},
		ZoomDist: originalZoomDist,
	}

	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
		thumbSize := thumbnail.Fit(originalSize)
		bitmaps[1+i] = BitmapAtZoom{
			Bitmap: Bitmap{
				Path: thumbnail.GetPath(originalPath),
				Sprite: Sprite{
					Rect: photo.Sprite.Rect,
				},
			},
			ZoomDist: photo.Sprite.Rect.GetPixelZoomDist(c, thumbSize),
		}
		// fmt.Printf("orig w %4.0f h %4.0f   thumb w %4.0f h %4.0f   zoom dist best %8.2f cur %8.2f area %8.6f\n", originalSize.Width, originalSize.Height, thumbSize.Width, thumbSize.Height, bestZoomDist, zoomDist, photo.Original.Sprite.Rect.GetPixelArea(c, thumbSize))
	}

	sort.Slice(bitmaps, func(i, j int) bool {
		a := bitmaps[i]
		b := bitmaps[j]
		return a.ZoomDist < b.ZoomDist
	})

	return bitmaps
}

func (photo *Photo) Draw(config *RenderConfig, scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) {

	if photo.Sprite.IsVisible(c, scales) {

		pixelArea := photo.Sprite.Rect.GetPixelArea(c, Size{X: 1, Y: 1})
		if pixelArea < config.MaxSolidPixelArea {
			style := c.Style

			info := source.GetImageInfo(source.GetImagePath(photo.Id))
			style.FillColor = info.GetColor()

			photo.Sprite.DrawWithStyle(c, style)
			return
		}

		drawn := false
		bitmaps := photo.getBestBitmaps(config, scene, c, scales, source)
		for _, bitmapAtZoom := range bitmaps {
			bitmap := bitmapAtZoom.Bitmap

			// text := fmt.Sprintf("index %d zd %4.2f %s", index, bitmapAtZoom.ZoomDist, bitmap.Path)
			// println(text)

			err := bitmap.Draw(config.CanvasImage, c, scales, source)
			if err == nil {
				drawn = true

				if config.DebugOverdraw {
					bitmap.DrawOverdraw(c, source)
				}

				if config.DebugThumbnails {
					text := ""

					for i := range source.Thumbnails {
						thumbnail := &source.Thumbnails[i]
						thumbnailPath := thumbnail.GetPath(source.GetImagePath(photo.Id))
						if source.Exists(thumbnailPath) {
							text += thumbnail.Name + " "
						}
					}

					bitmap.Sprite.DrawText(c, scales, &scene.Fonts.Debug, text)
				}

				break
			}

			// bitmap.Sprite.DrawText(c, scales, &scene.Fonts.Debug, text)
		}

		if !drawn {
			style := c.Style
			style.FillColor = canvas.Red
			photo.Sprite.DrawWithStyle(c, style)
		}

	}

}

func (solid *Solid) Draw(c *canvas.Context, scales Scales) {
	if solid.Sprite.IsVisible(c, scales) {
		prevFill := c.FillColor
		c.SetFillColor(solid.Color)
		solid.Sprite.Draw(c)
		c.SetFillColor(prevFill)
	}
}

func (scene *Scene) getRegionScale() float64 {
	return scene.Bounds.W
}

func (scene *Scene) GetRegions(config *RenderConfig, bounds Rect) []Region {
	scale := scene.getRegionScale()
	rect := bounds.Scale(scale)
	regions := scene.RegionSource.GetRegionsFromBounds(
		rect,
		scene,
		RegionConfig{
			Limit: 100,
		},
	)
	for i := range regions {
		region := &regions[i]
		region.Bounds = region.Bounds.Scale(1 / scale)
	}
	return regions
}

func (scene *Scene) GetRegion(id int) Region {
	scale := scene.getRegionScale()
	region := scene.RegionSource.GetRegionById(id, scene, RegionConfig{})
	region.Bounds = region.Bounds.Scale(1 / scale)
	return region
}
