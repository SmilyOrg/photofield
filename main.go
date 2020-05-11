package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unsafe"

	decoder "photofield/src"

	"github.com/dgraph-io/ristretto"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"
)

var fontFamily *canvas.FontFamily
var textFace canvas.FontFace

// var img image.Image
var mainScene Scene
var mainConfig Config

var imageSource *ImageSource

type Thumbnail struct {
	pathTemplate *template.Template
	size         Size
}

func NewThumbnail(pathTemplate string, size Size) Thumbnail {
	template, err := template.New("").Parse(pathTemplate)
	if err != nil {
		panic(err)
	}
	return Thumbnail{
		pathTemplate: template,
		size:         size,
	}
}

type Config struct {
	tileSize   int
	thumbnails []Thumbnail
}

type Scene struct {
	size   Size
	photos []Photo
}

type Scales struct {
	pixel float64
	tile  float64
}

type TileWriter func(w io.Writer) error

type Transform struct {
	view canvas.Matrix
}

type Sprite struct {
	transform Transform
	size      Size
}
type Size struct {
	width, height float64
}

type Bitmap struct {
	sprite Sprite
	path   string
	// image  *image.Image
}

type Photo struct {
	original Bitmap
	bitmaps  []Bitmap
	solid    Sprite
	Dir      string
	Filename string
}

type ImageRef struct {
	path  string
	image *image.Image
	// mutex LoadingImage
	// LoadingImage.loadOnce
}

type LoadingImage struct {
	imageRef *ImageRef
	mutex    sync.RWMutex
	// mutex sync.Mutex
	// cond  *sync.Cond
}

type ImageConfigRef struct {
	config image.Config
}

type ImageSource struct {
	imagesLoading sync.Map
	images        *ristretto.Cache
	configs       *ristretto.Cache
	// imageByPath       sync.Map
	// imageConfigByPath sync.Map
}

func NewImageSource() *ImageSource {
	var err error
	source := ImageSource{}
	source.images, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // number of keys to track frequency of (10M).
		// MaxCost:     1 << 30, // maximum cost of cache
		MaxCost:     1 << 27, // maximum cost of cache
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			imageRef := value.(ImageRef)
			// config := source.GetImageConfig(imageRef.path)
			// switch imageType := (*imageRef.image).(type) {
			// case image.YCbCr:
			// 	println("YCbCr")
			// default:
			// 	println("UNKNOWN")
			// }
			// return 1
			ycbcr, ok := (*imageRef.image).(*image.YCbCr)
			if !ok {
				fmt.Println("Unable to compute cost, unsupported image format")
				return 1
				// panic("Unable to compute cost, unsupported image format")
			}
			// fmt.Printf("%s %d %d %d %d %d\n", imageRef.path, unsafe.Sizeof(*ycbcr), unsafe.Sizeof(ycbcr.Y[0]), cap(ycbcr.Y), cap(ycbcr.Cb), cap(ycbcr.Cr))
			bytes := int64(unsafe.Sizeof(*ycbcr)) +
				int64(cap(ycbcr.Y))*int64(unsafe.Sizeof(ycbcr.Y[0])) +
				int64(cap(ycbcr.Cb))*int64(unsafe.Sizeof(ycbcr.Cb[0])) +
				int64(cap(ycbcr.Cr))*int64(unsafe.Sizeof(ycbcr.Cr[0]))
			// fmt.Printf("%s %d\n", imageRef.path, bytes)
			return bytes
			// return unsafe.Sizeof(image.RGBA) + config.Width*config.Height
		},
	})
	source.configs, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 27, // maximum cost of cache (128MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	return &source
}

func (source *ImageSource) LoadImage(path string) (*image.Image, error) {
	fmt.Printf("loading %s\n", path)
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	image, _, err := decoder.Decode(file)
	return &image, err
}

func (source *ImageSource) LoadConfig(path string) (image.Config, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return image.Config{}, err
	}
	config, _, err := decoder.DecodeConfig(file)
	return config, err
}

func (source *ImageSource) CacheImage(path string) (*image.Image, error) {
	image, err := source.LoadImage(path)
	source.images.Set(path, ImageRef{
		path:  path,
		image: image,
	}, 0)
	return image, err
}

func (source *ImageSource) GetImage(path string) (*image.Image, error) {
	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
	// 	source.images.Metrics.Ratio()*100,
	// 	source.images.Metrics.CostAdded()/1024/1024,
	// 	source.images.Metrics.CostEvicted()/1024/1024,
	// 	source.images.Metrics.Hits(),
	// 	source.images.Metrics.Misses())
	tries := 1000
	for try := 0; try < tries; try++ {
		value, found := source.images.Get(path)
		if found {
			return value.(ImageRef).image, nil
		} else {
			loadingImage := &LoadingImage{}
			// loadingImage.cond = sync.NewCond(&loadingImage.mutex)
			loadingImage.mutex.Lock()
			stored, loaded := source.imagesLoading.LoadOrStore(path, loadingImage)
			if loaded {
				// loadingImage.mutex.Unlock()
				// loadingImage = stored.(*LoadingImage)
				// loadingImage.mutex.Lock()
				// if loadingImage.cond != nil {
				// 	log.Printf("%v not found, try %v, waiting\n", path, try)
				// 	loadingImage.cond.Wait()
				// 	log.Printf("%v not found, try %v, waiting done\n", path, try)
				// 	imageRef := loadingImage.imageRef
				// 	loadingImage.mutex.Unlock()
				// 	return imageRef.image, nil
				// } else {
				// 	log.Printf("%v not found, try %v, done (no cond)\n", path, try)
				// 	return loadingImage.imageRef.image, nil
				// }
				loadingImage.mutex.Unlock()
				loadingImage = stored.(*LoadingImage)
				// log.Printf("%v not found, try %v, waiting load, mutex rlocked\n", path, try)
				loadingImage.mutex.RLock()
				// log.Printf("%v not found, try %v, waiting done, mutex runlocked\n", path, try)
				imageRef := loadingImage.imageRef
				loadingImage.mutex.RUnlock()
				if imageRef == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				return imageRef.image, nil
			} else {

				// log.Printf("%v not found, try %v, loading, mutex locked\n", path, try)
				image, err := source.LoadImage(path)
				if err != nil {
					panic(err)
				}

				imageRef := ImageRef{
					path:  path,
					image: image,
				}
				source.images.Set(path, imageRef, 0)
				loadingImage.imageRef = &imageRef
				// log.Printf("%v not found, try %v, loaded, broadcast\n", path, try)
				// cond := loadingImage.cond
				// loadingImage.cond = nil
				// cond.Broadcast()

				// source.imagesLoading.Delete(path)
				// log.Printf("%v not found, try %v, loaded, mutex unlocked\n", path, try)
				loadingImage.mutex.Unlock()

				return image, nil
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("Unable to get image after %v tries", tries))

	// imageRef := &ImageRef{}
	// imageRef.mutex.Lock()
	// stored, loaded := source.imageByPath.LoadOrStore(path, imageRef)

	// var loadedImage *image.Image

	// if loaded {
	// 	imageRef.mutex.Unlock()
	// 	imageRef = stored.(*ImageRef)
	// 	imageRef.mutex.RLock()
	// 	loadedImage = imageRef.image
	// 	imageRef.mutex.RUnlock()
	// } else {
	// 	image, err := source.LoadImage(path)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	imageRef.image = image
	// 	loadedImage = imageRef.image
	// 	imageRef.mutex.Unlock()
	// }

	// return loadedImage
}

func (source *ImageSource) GetImageConfig(path string) image.Config {
	value, found := source.configs.Get(path)
	if found {
		return value.(image.Config)
	} else {
		config, err := source.LoadConfig(path)
		if err != nil {
			panic(err)
		}
		source.configs.Set(path, config, 1)
		return config
	}
}

// func (source *ImageSource) GetImageConfig(path string) *image.Config {
// 	configRef := &ImageConfigRef{}
// 	stored, loaded := source.imageConfigByPath.LoadOrStore(path, configRef)
// 	configRef = stored.(*ImageConfigRef)
// 	if !loaded {
// 		file, err := os.Open(path)
// 		if err != nil {
// 			panic(err)
// 		}
// 		defer file.Close()
// 		config, _, err := decoder.DecodeConfig(file)
// 		if err != nil {
// 			panic(err)
// 		}
// 		configRef.config = config
// 	}
// 	return &configRef.config
// }

func (t *Transform) Push(c *canvas.Context) {
	c.Push()
	c.ComposeView(t.view)
}

func (t *Transform) Pop(c *canvas.Context) {
	c.Pop()
}

// func (bitmap *Bitmap) ensureImage() {

// 	if bitmap.image != nil {
// 		return
// 	}

// 	bitmap.image = imageSource.GetImage(bitmap.path)

// file, err := os.Open(bitmap.path)
// if err != nil {
// 	panic(err)
// }

// image, err := jpeg.Decode(file)
// if err != nil {
// 	panic(err)
// }

// bitmap.image = &image
// }

func (bitmap *Bitmap) Draw(c *canvas.Context, scales Scales) {
	// bitmap.sprite.transform.Push(c)
	// defer bitmap.sprite.transform.Pop(c)
	// bitmap.ensureImage()

	if bitmap.sprite.IsVisible(c, scales) {
		image, err := imageSource.GetImage(bitmap.path)
		if err != nil {
			panic(err)
		}
		c.RenderImage(*image, c.View().Mul(bitmap.sprite.transform.view))
	} else {
		c.Push()
		c.SetFillColor(canvas.Red)
		bitmap.sprite.Draw(c)
		c.Pop()
	}

	// c.RenderPath(canvas.Rectangle(bitmap.sprite.size.width, bitmap.sprite.size.height), c.Style, c.View().Mul(bitmap.sprite.transform.view))
}

func (bitmap *Bitmap) GetSize() image.Point {
	config := imageSource.GetImageConfig(bitmap.path)
	return image.Point{X: config.Width, Y: config.Height}
}

func (sprite *Sprite) Place(
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

	y += -fitHeight + scale*contentHeight

	sprite.size = Size{width: contentWidth, height: contentHeight}
	sprite.transform.view = canvas.Identity.
		Translate(x, -fitHeight-y).
		Scale(scale, scale)
}

func (photo *Photo) Place(x float64, y float64, width float64, height float64) {
	imageSize := photo.original.GetSize()
	imageWidth := float64(imageSize.X)
	imageHeight := float64(imageSize.Y)

	photo.solid.Place(x, y, width, height, imageWidth, imageHeight)
	photo.original.sprite.Place(x, y, width, height, imageWidth, imageHeight)
	for i := range photo.bitmaps {
		bitmap := &photo.bitmaps[i]
		imageSize := bitmap.GetSize()
		imageWidth := float64(imageSize.X)
		imageHeight := float64(imageSize.Y)
		bitmap.sprite.Place(x, y, width, height, imageWidth, imageHeight)

		// px, py := thumb.sprite.transform.view.Pos()
		// fmt.Printf("%v %v %v\n", width, imageWidth, thumb.sprite.size.width)
	}

	// photo.solid.size = Size{width: scale * imageWidth, height: scale * imageHeight}
	// photo.solid.transform.view = canvas.Identity.
	// 	Translate(x, y)
}

func (sprite *Sprite) Draw(c *canvas.Context) {
	c.RenderPath(canvas.Rectangle(sprite.size.width, sprite.size.height), c.Style, c.View().Mul(sprite.transform.view))
}

func (sprite *Sprite) DrawText(c *canvas.Context, x float64, y float64, size float64, text string) {
	px, py := sprite.transform.view.Pos()
	px += x
	py += y
	matrix := c.View().Translate(px, py)
	textFace := fontFamily.Face(size, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)
	// textBox := canvas.NewTextBox(textFace, text, sprite.size.width, sprite.size.height, canvas.Justify, canvas.Top, 5.0, 0.0)
	textBox := canvas.NewTextLine(textFace, text, canvas.Left)
	c.RenderText(textBox, matrix)
}

func (sprite *Sprite) DrawDebugOverlay(c *canvas.Context, scales Scales) {
	c.Push()
	pixelZoom := sprite.GetPixelZoom(c, scales)
	barWidth := -pixelZoom * 0.1
	// barHeight := 0.04
	alpha := pixelZoom * 0.1 * 0xFF
	max := 0.8 * float64(0xFF)
	if barWidth > 0 {
		alpha = math.Min(max, math.Max(0, -alpha))
		c.SetFillColor(color.NRGBA{0xFF, 0x00, 0x00, uint8(alpha)})
	} else {
		alpha = math.Min(max, math.Max(0, alpha))
		c.SetFillColor(color.NRGBA{0x00, 0x00, 0xFF, uint8(alpha)})
	}
	// c.RenderPath(
	// 	canvas.Rectangle(sprite.size.width*0.5*barWidth, sprite.size.height*barHeight),
	// 	c.Style,
	// 	c.View().Mul(sprite.transform.view).Translate(sprite.size.width*0.5, sprite.size.height*(0.5-barHeight*0.5)),
	// )

	sprite.Draw(c)
	// sprite.DrawText(c, 10, 10, 100*scales.pixel, fmt.Sprintf("Pixel zoom: %v", pixelZoom))

	// drawText(c, 30.0, canvas.NewTextBox(textFace, "Hello", 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))

	c.Pop()
}

func (sprite *Sprite) IsVisible(c *canvas.Context, scales Scales) bool {
	rect := canvas.Rect{X: 0, Y: 0, W: sprite.size.width, H: sprite.size.height}
	canvasToUnit := canvas.Identity.
		Scale(scales.tile, scales.tile).
		Mul(c.View().Mul(sprite.transform.view))
	unitRect := rect.Transform(canvasToUnit)
	return unitRect.X <= 1 && unitRect.Y <= 1 && unitRect.X+unitRect.W >= 0 && unitRect.Y+unitRect.H >= 0
}

func (sprite *Sprite) GetTileArea(scales Scales) float64 {
	return sprite.size.width * sprite.size.height * scales.pixel * scales.pixel
}

func (sprite *Sprite) GetPixelArea(c *canvas.Context, scales Scales) float64 {
	rect := canvas.Rect{X: 0, Y: 0, W: 1, H: 1}
	canvasToTile := c.View().Mul(sprite.transform.view)
	tileRect := rect.Transform(canvasToTile)
	area := tileRect.W * tileRect.H
	return area
}

func (sprite *Sprite) GetPixelZoom(c *canvas.Context, scales Scales) float64 {
	pixelArea := sprite.GetPixelArea(c, scales)
	if pixelArea >= 1 {
		return pixelArea
	} else {
		return -1 / pixelArea
	}
	// if zoom < 0 {
	// 	zoom = 1 / zoom
	// }
	// return zoom
}

func (photo *Photo) SetImagePath(path string) {
	photo.original.path = path
	dir, filename := filepath.Split(path)
	photo.Dir = dir
	photo.Filename = filename

	small := Bitmap{
		path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_S.jpg", dir, filename),
	}
	photo.bitmaps = append(photo.bitmaps, small)

	medium := Bitmap{
		path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_M.jpg", dir, filename),
	}
	photo.bitmaps = append(photo.bitmaps, medium)

	big := Bitmap{
		path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_B.jpg", dir, filename),
	}
	photo.bitmaps = append(photo.bitmaps, big)

	xl := Bitmap{
		path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_XL.jpg", dir, filename),
	}
	photo.bitmaps = append(photo.bitmaps, xl)

	photo.bitmaps = append(photo.bitmaps, photo.original)
}

// func (sprite *Sprite) GetPixelArea(c *canvas.Context, scales Scales) float64 {
// 	rect := canvas.Rect{X: 0, Y: 0, W: 1, H: 1}
// 	canvasToTile := c.View().Mul(sprite.transform.view)
// 	tileRect := rect.Transform(canvasToTile)
// 	area := tileRect.W * tileRect.H
// 	return area
// }

// func (sprite *Sprite) GetPixelZoom(c *canvas.Context, scales Scales) float64 {
// 	pixelArea := sprite.GetPixelArea(c, scales)
// 	if pixelArea >= 1 {
// 		return pixelArea
// 	} else {
// 		return -1 / pixelArea
// 	}
// 	// if zoom < 0 {
// 	// 	zoom = 1 / zoom
// 	// }
// 	// return zoom
// }

func (sprite *Sprite) GetPixelAreaThumb(c *canvas.Context, size Size) float64 {
	rect := canvas.Rect{X: 0, Y: 0, W: 1, H: 1}
	canvasToTile := c.View().Mul(sprite.transform.view).
		Scale(
			sprite.size.width/size.width,
			sprite.size.height/size.height,
		)
	tileRect := rect.Transform(canvasToTile)
	area := tileRect.W * tileRect.H
	return area
}

func (sprite *Sprite) GetPixelZoomThumb(c *canvas.Context, size Size) float64 {
	pixelArea := sprite.GetPixelAreaThumb(c, size)
	if pixelArea >= 1 {
		return pixelArea
	} else {
		return -1 / pixelArea
	}
}

func (sprite *Sprite) GetPixelZoomDistThumb(c *canvas.Context, size Size) float64 {
	return math.Abs(sprite.GetPixelZoomThumb(c, size))
}

func (photo *Photo) getBestBitmap(config *Config, c *canvas.Context, scales Scales) *Bitmap {
	var best *Thumbnail
	bestZoomDist := photo.original.sprite.GetPixelZoomDistThumb(c, photo.original.sprite.size)
	for i := range config.thumbnails {
		thumbnail := &config.thumbnails[i]
		zoomDist := photo.original.sprite.GetPixelZoomDistThumb(c, thumbnail.size)
		if zoomDist < bestZoomDist {
			best = thumbnail
			bestZoomDist = zoomDist
		}
	}
	if best == nil {
		return &photo.original
	}
	var rendered bytes.Buffer
	err := best.pathTemplate.Execute(&rendered, photo)
	if err != nil {
		panic(err)
	}
	return &Bitmap{
		path: rendered.String(),
		sprite: Sprite{
			size: Size{
				width:  best.size.width,
				height: best.size.height,
			},
			transform: Transform{
				view: photo.original.sprite.transform.view.Scale(
					photo.original.sprite.size.width/best.size.width,
					photo.original.sprite.size.height/best.size.height,
				),
			},
		},
	}
}

func (photo *Photo) Draw(config *Config, c *canvas.Context, scales Scales) {
	// photo.original.Draw(c)

	// c.Push()

	// if photo.solid.IsVisible(c, scales) {
	// c.SetFillColor(canvas.Black)
	// 	println("visible")
	// } else {
	// 	c.SetFillColor(canvas.Red)
	// 	println("invisible")
	// }

	if photo.original.sprite.IsVisible(c, scales) {

		// var best *Bitmap
		// bestZoomDist := math.Inf(1)
		// for i := range photo.bitmaps {
		// 	bitmap := &photo.bitmaps[i]
		// 	zoom := bitmap.sprite.GetPixelZoom(c, scales)
		// 	zoomDist := math.Abs(zoom)
		// 	if zoomDist < bestZoomDist {
		// 		best = bitmap
		// 		bestZoomDist = zoomDist
		// 	}
		// }

		best := photo.getBestBitmap(config, c, scales)

		if best != nil {
			bitmap := best
			bitmap.Draw(c, scales)
			// bitmap.sprite.DrawDebugOverlay(c, scales)
			// bitmap.sprite.DrawText(c, 0, 0, 100, filepath.Base(bitmap.path))
			// bitmap.sprite.DrawText(c, 0, 0, 100, fmt.Sprintf("%d %.2f", bestIndex, bestZoomDist))
		}
		// photo.original.Draw(c)
		// photo.solid.Draw(c)
	} else {
		c.Push()
		c.SetFillColor(canvas.Red)
		photo.original.sprite.Draw(c)
		c.Pop()
	}

	// photo.solid.Draw(c)

	// c.Pop()

	// fmt.Printf("%f\n", photo.solid.GetPixelArea(pixelScale))
}

func drawTile(c *canvas.Context, config *Config, scene *Scene, zoom int, x int, y int) {

	tileSize := float64(config.tileSize)
	zoomPower := 1 << zoom

	// println(zoomPower, x, y)
	// edgeTiles := scale

	tx := float64(x) * tileSize
	ty := float64(zoomPower-1-y) * tileSize

	// fitScale := scene.size.width / tileSize

	var scale float64
	if tileSize/tileSize < scene.size.width/scene.size.height {
		scale = tileSize / scene.size.width
		ty += (scale*scene.size.height - tileSize) * 0.5
	} else {
		scale = tileSize / scene.size.height
		tx += (scale*scene.size.width - tileSize) * 0.5
	}

	scale *= float64(zoomPower)

	scales := Scales{
		pixel: scale,
		tile:  1 / float64(tileSize),
	}

	// +tileSize*float64(zoomPower)

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)
	c.SetFillColor(canvas.White)
	c.DrawPath(0, 0, canvas.Rectangle(scene.size.width, -scene.size.height))

	c.SetFillColor(canvas.Black)

	// headerFace := fontFamily.Face(28.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	// textFace := fontFamily.Face(12.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	// drawText(c, 30.0, canvas.NewTextBox(headerFace, "Document Example", 0.0, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[0], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))

	// p := &canvas.Path{}

	// p.MoveTo(100, 100)
	// p := canvas.Circle(50)

	// c.DrawImage(0, 0, img, 5000)

	// for iy := 0; iy < 11; iy++ {
	// 	for ix := 0; ix < 11; ix++ {
	// 		c.DrawPath(0.1*float64(ix), 0.1*float64(iy), canvas.Circle(0.01))
	// 	}
	// }

	// gridNum := 10
	// for iy := 0; iy < gridNum; iy++ {
	// 	for ix := 0; ix < gridNum; ix++ {
	// 		x := float64(ix) / float64(gridNum-1) * scene.size.width
	// 		y := float64(iy) / float64(gridNum-1) * scene.size.height
	// 		c.DrawPath(x, y, canvas.Circle(2))
	// 	}
	// }

	// for iy := 0; iy < 11; iy++ {
	// 	for ix := 0; ix < 11; ix++ {
	// 		c.DrawPath(100*float64(ix), -100*float64(iy), canvas.Circle(5))
	// 	}
	// }

	// c.DrawPath(0.0*scene.size.width, -0.0*scene.size.height, canvas.Circle(10))
	// c.DrawPath(0.0*scene.size.width, -1.0*scene.size.height, canvas.Circle(10))
	// c.DrawPath(1.0*scene.size.width, -0.0*scene.size.height, canvas.Circle(10))
	// c.DrawPath(1.0*scene.size.width, -1.0*scene.size.height, canvas.Circle(10))

	// c.DrawPath(0.1, 0.1, canvas.Circle(0.1))
	// c.DrawPath(0.1, 0.9, canvas.Circle(0.1))
	// c.DrawPath(0.9, 0.1, canvas.Circle(0.1))
	// c.DrawPath(0.9, 0.9, canvas.Circle(0.1))

	for _, photo := range scene.photos {
		photo.Draw(config, c, scales)
	}

	// photo := scene.photos[4]
	// for i := range photo.bitmaps {
	// 	bitmap := &photo.bitmaps[i]
	// 	bitmap.sprite.transform.view = canvas.Identity.
	// 		Translate(10, -float64(i)*100).
	// 		Scale(0.01, 0.01)
	// 	bitmap.Draw(c, scales)
	// }

	// c.ComposeView(canvas.Identity.Scale(1, -1))

	// n := 50
	// cx := 0.1
	// for i := 0; i < n; i++ {
	// 	radius := math.Pow10(-1 - i)
	// 	cx += radius
	// 	c.DrawPath(cx, 0.5, canvas.Circle(radius))
	// 	cx += radius
	// }

	// headerFace := fontFamily.Face(28.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	// drawText(c, 0.0, canvas.NewTextBox(headerFace, "Document Example", 0.0, 0.0, 1, 1, 0.0, 0.0))

	// c.DrawPath(0.2, 0.3, canvas.Circle(0.1))
	// c.DrawPath(0.3, 0.3, canvas.Circle(0.05))
	// c.DrawPath(0.4, 0.3, canvas.Circle(0.01))
	// c.DrawPath(0.5, 0.3, canvas.Circle(0.005))
	// c.DrawPath(0.6, 0.3, canvas.Circle(0.001))
	// c.DrawPath(0.7, 0.3, canvas.Circle(0.0005))

	// lenna, err := os.Open("../lenna.png")
	// if err != nil {
	// 	panic(err)
	// }
	// img, err := png.Decode(lenna)
	// if err != nil {
	// 	panic(err)
	// }
	// imgDPM := 15.0
	// imgWidth := float64(img.Bounds().Max.X) / imgDPM
	// imgHeight := float64(img.Bounds().Max.Y) / imgDPM
	// c.DrawImage(170.0-imgWidth, y-imgHeight, img, imgDPM)

	// imgWidth := 50.
	// imgHeight := 50.

	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[1], 140.0-imgWidth-10.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[2], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
	//drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[3], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
}

func getTileCanvas(config *Config, scene *Scene, zoom int, x int, y int) *canvas.Canvas {
	c := canvas.New(float64(config.tileSize), float64(config.tileSize))
	ctx := canvas.NewContext(c)
	drawTile(ctx, config, scene, zoom, x, y)
	return c
}

func tilesHandler(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])

	query := r.URL.Query()

	config := mainConfig

	tileSizeQuery, err := strconv.Atoi(query.Get("tileSize"))
	if err == nil && tileSizeQuery > 0 {
		config.tileSize = tileSizeQuery
	}

	zoom, err := strconv.Atoi(query.Get("zoom"))
	if err != nil {
		http.Error(w, "Invalid zoom", http.StatusBadRequest)
		return
	}

	x, err := strconv.Atoi(query.Get("x"))
	if err != nil {
		http.Error(w, "Invalid x", http.StatusBadRequest)
		return
	}

	y, err := strconv.Atoi(query.Get("y"))
	if err != nil {
		http.Error(w, "Invalid y", http.StatusBadRequest)
		return
	}

	c := getTileCanvas(&config, &mainScene, zoom, x, y)
	rasterizer.PNGWriter(1.0)(w, c)

	// rasterizer.Draw(c *Canvas, resolution DPMM)
	// c.WriteFile("out.png", rasterizer.PNGWriter(5.0))
	// getTilePngWriter()(w)
}

func main() {

	imageSource = NewImageSource()

	fontFamily = canvas.NewFontFamily("sans")
	// fontFamily.Use(canvas.CommonLigatures)
	if err := fontFamily.LoadLocalFont("sans", canvas.FontRegular); err != nil {
		panic(err)
	}
	textFace = fontFamily.Face(48.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)

	// photo, err := os.Open("P1110271.JPG")
	// if err != nil {
	// 	panic(err)
	// }

	// image, err := jpeg.Decode(photo)
	// if err != nil {
	// 	panic(err)
	// }

	// photoCount := 697
	photoCount := 50
	// var photoDirs = "./photos"
	var photoDirs = []string{
		"/mnt/d/photos/copy/USA 2018/Lumix/100_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/101_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/102_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/103_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/104_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/105_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/106_PANA",
	}
	// var photoPath = "/mnt/p/Moments/USA 2018/Cybershot/100MSDCF"
	// var photoPath = "/mnt/p/Moments/USA 2018/Lumix/100_PANA"
	// var photoPath = "/mnt/d/photos/resized/USA 2018/Lumix/100_PANA/"
	var photoFilePaths []string

	for _, photoDir := range photoDirs {
		filepath.Walk(photoDir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if strings.Contains(path, "@eaDir") {
					return filepath.SkipDir
				}
				if !strings.HasSuffix(strings.ToLower(path), ".jpg") {
					return nil
				}
				fmt.Printf("adding %s\n", path)
				photoFilePaths = append(photoFilePaths, path)
				if len(photoFilePaths) >= photoCount {
					return errors.New("Skipping the rest")
				}
				return nil
			},
		)
	}

	// if err != nil {
	// 	log.Println(err)
	// }

	// files, err := ioutil.ReadDir("./photos/Trip_Wuhletal")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for _, file := range files {
	// 	name := file.Name()
	// 	if strings.HasSuffix(strings.ToLower(name), ".jpg") {
	// 		photoPaths = append(photoPaths, name)
	// 	}
	// }

	// small := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_S.jpg", dir, filename),
	// }
	// photo.bitmaps = append(photo.bitmaps, small)

	// medium := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_M.jpg", dir, filename),
	// }
	// photo.bitmaps = append(photo.bitmaps, medium)

	// big := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_B.jpg", dir, filename),
	// }
	// photo.bitmaps = append(photo.bitmaps, big)

	// xl := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_XL.jpg", dir, filename),
	// }

	scene := &mainScene
	mainConfig.tileSize = 256
	mainConfig.thumbnails = []Thumbnail{
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_S.jpg",
			Size{width: 120, height: 80},
		),
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_M.jpg",
			Size{width: 480, height: 320},
		),
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_B.jpg",
			Size{width: 640, height: 427},
		),
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_XL.jpg",
			Size{width: 1920, height: 1280},
		),
	}

	log.Println("placing")

	config := mainConfig
	config.tileSize = 400
	scene.size = Size{width: 1000, height: 1000}

	imageWidth := 120.
	// imageWidth := 14.
	imageHeight := imageWidth * 2 / 3
	margin := 1.
	cols := int(scene.size.width/(imageWidth+margin)) - 2

	// scene.size = Size{width: 210, height: 297}
	// scene.size = Size{width: 297, height: 210}
	scene.photos = make([]Photo, photoCount)
	for i := 0; i < photoCount; i++ {
		// scene.photos[i] = Bitmap{
		// 	sprite: Sprite{
		// 		transform: Transform{
		// 			view: canvas.Identity.
		// 				Translate(100, 100),
		// 			// Scale(1+10*float64(i), 1+10*float64(i)),
		// 		},
		// 		size: Size{
		// 			width:  300,
		// 			height: 300,
		// 		},
		// 	},
		// 	image: image,
		// }

		col := i % cols
		row := i / cols

		// scene.photos[i].SetImagePath("photos/P1110271.JPG")
		path := photoFilePaths[i]

		photo := &scene.photos[i]
		photo.SetImagePath(path)
		photo.Place((imageWidth+margin)*float64(1+col), (imageHeight+margin)*float64(1+row), imageWidth, imageHeight)
		log.Printf("placing %d / %d\n", i, photoCount)
	}

	// c := canvas.New(200, 200)
	// ctx := canvas.NewContext(c)
	// draw(ctx)

	log.Println("rendering sample")

	c := getTileCanvas(&config, scene, 0, 0, 0)
	c.WriteFile("out.png", rasterizer.PNGWriter(1.0))

	log.Println("serving")

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/tiles", tilesHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

var lorem = []string{
	`Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nulla malesuada fringilla libero vel ultricies. Phasellus eu lobortis lorem. Phasellus eu cursus mi. Sed enim ex, ornare et velit vitae, sollicitudin volutpat dolor. Sed aliquam sit amet nisi id sodales. Aliquam erat volutpat. In hac habitasse platea dictumst. Pellentesque luctus varius nibh sit amet porta. Vivamus tempus, enim ut sodales aliquet, magna massa viverra eros, nec gravida risus ipsum a erat. Etiam dapibus sem augue, at porta nisi dictum non. Vestibulum quis urna ut ligula dapibus mollis eu vel nisl. Vestibulum lorem dolor, eleifend lacinia fringilla eu, pulvinar vitae metus.`,
	`Morbi dapibus purus vel erat auctor, vehicula tempus leo maximus. Aenean feugiat vel quam sit amet iaculis. Fusce et justo nec arcu maximus porttitor. Cras sed aliquam ipsum. Sed molestie mauris nec dui interdum sollicitudin. Nulla id egestas massa. Fusce congue ante. Interdum et malesuada fames ac ante ipsum primis in faucibus. Praesent faucibus tellus eu viverra blandit. Vivamus mi massa, hendrerit in commodo et, luctus vitae felis.`,
	`Quisque semper aliquet augue, in dignissim eros cursus eu. Pellentesque suscipit consequat nibh, sit amet ultricies risus. Suspendisse blandit interdum tortor, consectetur tristique magna aliquet eu. Aliquam sollicitudin eleifend sapien, in pretium nisi. Sed tempor eleifend velit quis vulputate. Donec condimentum, lectus vel viverra pharetra, ex enim cursus metus, quis luctus est urna ut purus. Donec tempus gravida pharetra. Sed leo nibh, cursus at hendrerit at, ultricies a dui. Maecenas eget elit magna. Quisque sollicitudin odio erat, sed consequat libero tincidunt in. Nullam imperdiet, neque quis consequat pellentesque, metus nisl consectetur eros, ut vehicula dui augue sed tellus.`,
	//` Vivamus varius ex sed nisi vestibulum, sit amet tincidunt ante vestibulum. Nullam et augue blandit dolor accumsan tempus. Quisque at dictum elit, id ullamcorper dolor. Nullam feugiat mauris eu aliquam accumsan.`,
}

var y = 205.0

func drawText(c *canvas.Context, x float64, text *canvas.Text) {
	h := text.Bounds().H
	c.DrawText(x, y, text)
	y -= h + 10.0
}

// func draw(c *canvas.Context, int zoom, int x, int y) {
// 	c.SetView(canvas.Identity.Scale(2, 2).Translate(-100, -100))
// 	c.SetFillColor(canvas.White)
// 	c.DrawPath(0, 0, canvas.Rectangle(200, 200))

// 	c.SetFillColor(canvas.Black)

// 	// headerFace := fontFamily.Face(28.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
// 	// textFace := fontFamily.Face(12.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)

// 	// drawText(c, 30.0, canvas.NewTextBox(headerFace, "Document Example", 0.0, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
// 	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[0], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))

// 	// p := &canvas.Path{}

// 	// p.MoveTo(100, 100)
// 	// p := canvas.Circle(50)

// 	c.DrawPath(100, 100, canvas.Circle(50))

// 	// lenna, err := os.Open("../lenna.png")
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// img, err := png.Decode(lenna)
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// imgDPM := 15.0
// 	// imgWidth := float64(img.Bounds().Max.X) / imgDPM
// 	// imgHeight := float64(img.Bounds().Max.Y) / imgDPM
// 	// c.DrawImage(170.0-imgWidth, y-imgHeight, img, imgDPM)

// 	// imgWidth := 50.
// 	// imgHeight := 50.

// 	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[1], 140.0-imgWidth-10.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
// 	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[2], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
// 	//drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[3], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
// }
