package photofield

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"path/filepath"
	"sync"
	"text/template"
	"time"

	storage "photofield/internal/storage"

	"github.com/tdewolff/canvas"
	"golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

type Config struct {
	TileSize   int
	Thumbnails []Thumbnail
	LogDraws   bool
}

type ThumbnailSizeType int32

const (
	FitOutside ThumbnailSizeType = iota
	FitInside  ThumbnailSizeType = iota
)

type Thumbnail struct {
	pathTemplate *template.Template
	SizeType     ThumbnailSizeType
	Size         Size
}

func NewThumbnail(pathTemplate string, sizeType ThumbnailSizeType, size Size) Thumbnail {
	template, err := template.New("").Parse(pathTemplate)
	if err != nil {
		panic(err)
	}
	return Thumbnail{
		pathTemplate: template,
		SizeType:     sizeType,
		Size:         size,
	}
}

func (thumbnail *Thumbnail) GetPath(originalPath string) (string, error) {
	var rendered bytes.Buffer
	dir, filename := filepath.Split(originalPath)
	err := thumbnail.pathTemplate.Execute(&rendered, PhotoTemplateData{
		Dir:      dir,
		Filename: filename,
	})
	if err != nil {
		return "", err
	}
	return rendered.String(), nil
}

func (thumbnail *Thumbnail) Fit(originalSize Size) Size {
	thumbWidth, thumbHeight := thumbnail.Size.Width, thumbnail.Size.Height
	thumbRatio := thumbWidth / thumbHeight
	originalWidth, originalHeight := originalSize.Width, originalSize.Height
	originalRatio := originalWidth / originalHeight
	switch thumbnail.SizeType {
	case FitInside:
		if thumbRatio < originalRatio {
			thumbHeight = thumbWidth / originalRatio
		} else {
			thumbWidth = thumbHeight * originalRatio
		}
	case FitOutside:
		if thumbRatio > originalRatio {
			thumbHeight = thumbWidth / originalRatio
		} else {
			thumbWidth = thumbHeight * originalRatio
		}
	}
	return Size{
		Width:  math.Round(thumbWidth),
		Height: math.Round(thumbHeight),
	}
}

type PhotoTemplateData struct {
	Dir      string
	Filename string
}

type Transform struct {
	view canvas.Matrix
}

type Sprite struct {
	transform Transform
	size      Size
}
type Bitmap struct {
	Sprite Sprite
	Path   string
	// image  *image.Image
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

type Bounds struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Region struct {
	Id     int    `json:"id"`
	Bounds Bounds `json:"bounds"`
}

type Scene struct {
	Size         Size
	Solids       []Solid
	Photos       []Photo
	Texts        []Text
	RegionSource func(canvas.Rect, *Scene) []Region

	Canvas draw.Image
	Zoom   int
}

type Scales struct {
	Pixel float64
	Tile  float64
}

type Size struct {
	Width, Height float64
}

type Photo struct {
	Original Bitmap
	// bitmaps  []Bitmap
	// solid    Sprite
}

func NewSolidFromRect(rect canvas.Rect, color color.Color) Solid {
	solid := Solid{}
	solid.Color = color
	solid.Sprite.size = Size{Width: rect.W, Height: rect.H}
	solid.Sprite.transform.view = canvas.Identity.
		Translate(rect.X, -rect.H-rect.Y)
	return solid
}

func NewHeaderFromRect(rect canvas.Rect, font *canvas.FontFace, txt string) Text {
	text := Text{}
	text.Text = txt
	text.Font = font
	text.Sprite.size = Size{Width: rect.W, Height: rect.H}
	text.Sprite.transform.view = canvas.Identity.
		Translate(rect.X, -rect.Y)
	return text
}

func NewTextFromRect(rect canvas.Rect, font *canvas.FontFace, txt string) Text {
	text := Text{}
	text.Text = txt
	text.Font = font
	text.Sprite.size = Size{Width: rect.W, Height: rect.H}
	text.Sprite.transform.view = canvas.Identity.
		Translate(rect.X, -rect.Y-font.Metrics().Ascent)
	return text
}

func drawPhotos(photos []Photo, config *Config, scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) {
	for i := range photos {
		photo := &photos[i]
		photo.Draw(config, scene, c, scales, source)
	}
}

func drawPhotoChannel(id int, index chan int, config *Config, scene *Scene, c *canvas.Context, scales Scales, wg *sync.WaitGroup, source *storage.ImageSource) {

	var lastLogTime time.Time
	if config.LogDraws {
		lastLogTime = time.Now()
	}

	for i := range index {
		photo := &scene.Photos[i]
		if config.LogDraws {
			now := time.Now()
			if now.Sub(lastLogTime) > 1*time.Second {
				lastLogTime = now
				log.Printf("draw photo %d (goroutine %d)\n", i, id)
			}
		}
		photo.Draw(config, scene, c, scales, source)
	}

	wg.Done()
}

func (scene *Scene) Draw(config *Config, c *canvas.Context, scales Scales, source *storage.ImageSource) {
	for i := range scene.Solids {
		solid := &scene.Solids[i]
		solid.Draw(c, scales)
	}

	// for i := range scene.Photos {
	// 	photo := &scene.Photos[i]
	// 	photo.Draw(config, scene, c, scales, source)
	// }

	// concurrent := 1
	// photoCount := len(scene.Photos)
	// if photoCount < concurrent {
	// 	concurrent = photoCount
	// }

	index := make(chan int, 1)
	concurrent := 100

	wg := &sync.WaitGroup{}
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go drawPhotoChannel(i, index, config, scene, c, scales, wg, source)
	}
	for i := range scene.Photos {
		index <- i
	}
	close(index)
	wg.Wait()

	for i := range scene.Texts {
		text := &scene.Texts[i]
		text.Draw(c, scales)
	}
}

func (scene *Scene) GetVisiblePhotos(output chan *Photo, view canvas.Rect) {
	for i := range scene.Photos {
		photo := &scene.Photos[i]
		if photo.Original.Sprite.IsVisibleInRect(view) {
			output <- photo
		}
	}
	close(output)
}

func RenderImageFast(rimg draw.Image, img image.Image, m canvas.Matrix) {
	// add transparent margin to image for smooth borders when rotating
	// margin := 4
	// size := img.Bounds().Size()
	// sp := img.Bounds().Min // starting point
	// img2 := image.NewRGBA(image.Rect(0, 0, size.X+margin*2, size.Y+margin*2))
	// draw.Draw(img2, image.Rect(margin, margin, size.X+margin, size.Y+margin), img, sp, draw.Over)

	// resolution := 1

	// size := img.Bounds().Size()
	// sp := img.Bounds().Min // starting point

	// draw to destination image
	// note that we need to correct for the added margin in origin and m
	// TODO: optimize when transformation is only translation or stretch
	origin := m.Dot(canvas.Point{0, float64(img.Bounds().Size().Y)})
	// m = m.Scale((float64(size.X) / float64(size.X)), (float64(size.Y) / float64(size.Y)))

	h := float64(rimg.Bounds().Size().Y)
	aff3 := f64.Aff3{m[0][0], -m[0][1], origin.X, -m[1][0], m[1][1], h - origin.Y}
	// draw.CatmullRom.Transform(rimg, aff3, img, img.Bounds(), draw.Over, nil)
	// draw.NearestNeighbor.Transform(rimg, aff3, img, img.Bounds(), draw.Over, nil)
	// draw.ApproxBiLinear.Transform(rimg, aff3, img, img.Bounds(), draw.Over, nil)
	draw.ApproxBiLinear.Transform(rimg, aff3, img, img.Bounds(), draw.Src, nil)
	// draw.NearestNeighbor.Transform(rimg, aff3, img, img.Bounds(), draw.Src, nil)

	// margin := 4
	// size := img.Bounds().Size()
	// sp := img.Bounds().Min // starting point
	// img2 := image.NewRGBA(image.Rect(0, 0, size.X+margin*2, size.Y+margin*2))
	// draw.Draw(img2, image.Rect(margin, margin, size.X+margin, size.Y+margin), img, sp, draw.Over)

	// origin := m.Dot(canvas.Point{-float64(margin), float64(img2.Bounds().Size().Y - margin)}).Mul(float64(resolution))
	// m = m.Scale(float64(resolution)*(float64(size.X+margin)/float64(size.X)), float64(resolution)*(float64(size.Y+margin)/float64(size.Y)))
	// h := float64(rimg.Bounds().Size().Y)
	// aff3 := f64.Aff3{m[0][0], -m[0][1], origin.X, -m[1][0], m[1][1], h - origin.Y}
	// draw.CatmullRom.Transform(rimg, aff3, img2, img2.Bounds(), draw.Over, nil)

	// draw to destination image
	// note that we need to correct for the added margin in origin and m
	// TODO: optimize when transformation is only translation or stretch
	// origin := m.Dot(canvas.Point{-float64(margin), float64(img2.Bounds().Size().Y - margin)}).Mul(float64(resolution))
	// m = m.Scale(float64(resolution)*(float64(size.X+margin)/float64(size.X)), float64(resolution)*(float64(size.Y+margin)/float64(size.Y)))

	// h := float64(rimg.Bounds().Size().Y)
	// aff3 := f64.Aff3{m[0][0], -m[0][1], 0, -m[1][0], m[1][1], h - 0}
	// draw.CatmullRom.Transform(rimg, aff3, img2, img2.Bounds(), draw.Over, nil)

	// origin := m.Dot(canvas.Point{-float64(0), float64(img.Bounds().Size().Y - 0)}).Mul(float64(resolution))
	// 	m = m.Scale(float64(resolution)*(float64(size.X+0)/float64(size.X)), float64(resolution)*(float64(size.Y+0)/float64(size.Y)))

	// 	h := float64(rimg.Bounds().Size().Y)
	// 	aff3 := f64.Aff3{m[0][0], -m[0][1], origin.X, -m[1][0], m[1][1], h - origin.Y}

	// draw.CatmullRom.Transform(rimg, aff3, img, img.Bounds(), draw.Over, nil)
}

func (bitmap *Bitmap) Draw(scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) {
	if bitmap.Sprite.IsVisible(c, scales) {
		image, err := source.GetImage(bitmap.Path)
		if err != nil {
			panic(err)
		}

		// c.RenderImage(*image, c.View().Mul(bitmap.Sprite.transform.view))
		RenderImageFast(scene.Canvas, *image, c.View().Mul(bitmap.Sprite.transform.view))
	}
}

func (bitmap *Bitmap) GetSize(source *storage.ImageSource) image.Point {
	info := source.GetImageInfo(bitmap.Path)
	return image.Point{X: info.Width, Y: info.Height}
}

func (sprite *Sprite) PlaceFitHeight(
	x float64,
	y float64,
	fitHeight float64,
	contentWidth float64,
	contentHeight float64,
) {
	scale := fitHeight / contentHeight

	y += -fitHeight + scale*contentHeight

	sprite.size = Size{Width: contentWidth, Height: contentHeight}
	sprite.transform.view = canvas.Identity.
		Translate(x, -fitHeight-y).
		Scale(scale, scale)
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

	y += -fitHeight + scale*contentHeight

	sprite.size = Size{Width: contentWidth, Height: contentHeight}
	sprite.transform.view = canvas.Identity.
		Translate(x, -fitHeight-y).
		Scale(scale, scale)
}

func (photo *Photo) Place(x float64, y float64, width float64, height float64, source *storage.ImageSource) {
	imageSize := photo.Original.GetSize(source)
	imageWidth := float64(imageSize.X)
	imageHeight := float64(imageSize.Y)

	// photo.solid.Place(x, y, width, height, imageWidth, imageHeight)
	photo.Original.Sprite.PlaceFit(x, y, width, height, imageWidth, imageHeight)
	// for i := range photo.bitmaps {
	// 	bitmap := &photo.bitmaps[i]
	// 	imageSize := bitmap.GetSize()
	// 	imageWidth := float64(imageSize.X)
	// 	imageHeight := float64(imageSize.Y)
	// 	bitmap.sprite.Place(x, y, width, height, imageWidth, imageHeight)

	// 	// px, py := thumb.sprite.transform.view.Pos()
	// 	// fmt.Printf("%v %v %v\n", width, imageWidth, thumb.sprite.size.width)
	// }

	// photo.solid.size = Size{width: scale * imageWidth, height: scale * imageHeight}
	// photo.solid.transform.view = canvas.Identity.
	// 	Translate(x, y)
}

func (sprite *Sprite) Draw(c *canvas.Context) {
	c.RenderPath(canvas.Rectangle(sprite.size.Width, sprite.size.Height), c.Style, c.View().Mul(sprite.transform.view))
}

func (sprite *Sprite) DrawText(c *canvas.Context, x float64, y float64, size float64, text string) {
	// px, py := sprite.transform.view.Pos()
	// px += x
	// py += y
	// matrix := c.View().Translate(px, py)
	// textFace := fontFamily.Face(size, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)
	// textBox := canvas.NewTextBox(textFace, text, sprite.size.width, sprite.size.height, canvas.Justify, canvas.Top, 5.0, 0.0)
	// textBox := canvas.NewTextLine(textFace, text, canvas.Left)
	// c.RenderText(textBox, matrix)
}

func (text *Text) Draw(c *canvas.Context, scales Scales) {
	// px, py := sprite.transform.view.Pos()
	// px += x
	// py += y
	// textFace := fontFamily.Face(size, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)
	// textBox := canvas.NewTextBox(textFace, text, sprite.size.width, sprite.size.height, canvas.Justify, canvas.Top, 5.0, 0.0)
	// textBox := canvas.NewTextLine(textFace, text, canvas.Left)
	if text.Sprite.IsVisible(c, scales) {
		textLine := canvas.NewTextLine(*text.Font, text.Text, canvas.Left)
		c.RenderText(textLine, c.View().Mul(text.Sprite.transform.view))
	}
	// c.RenderPath(canvas.Rectangle(sprite.size.Width, sprite.size.Height), c.Style, c.View().Mul(sprite.transform.view))
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

func (sprite *Sprite) GetBounds() canvas.Rect {
	rect := canvas.Rect{X: 0, Y: 0, W: sprite.size.Width, H: sprite.size.Height}
	canvasToUnit := sprite.transform.view
	return rect.Transform(canvasToUnit)
}

func (sprite *Sprite) IsVisible(c *canvas.Context, scales Scales) bool {
	rect := canvas.Rect{X: 0, Y: 0, W: sprite.size.Width, H: sprite.size.Height}
	canvasToUnit := canvas.Identity.
		Scale(scales.Tile, scales.Tile).
		Mul(c.View().Mul(sprite.transform.view))
	unitRect := rect.Transform(canvasToUnit)
	return unitRect.X <= 1 && unitRect.Y <= 1 && unitRect.X+unitRect.W >= 0 && unitRect.Y+unitRect.H >= 0
}

func (sprite *Sprite) IsVisibleInRect(view canvas.Rect) bool {
	rect := canvas.Rect{X: 0, Y: 0, W: sprite.size.Width, H: sprite.size.Height}
	canvasRect := rect.Transform(sprite.transform.view)
	fmt.Println(view.String(), canvasRect.String())
	return canvasRect.X <= view.X+view.W && canvasRect.Y <= view.Y+view.H && canvasRect.X+canvasRect.W >= view.X && canvasRect.Y+canvasRect.H >= view.Y
}

func (sprite *Sprite) GetTileArea(scales Scales) float64 {
	return sprite.size.Width * sprite.size.Height * scales.Pixel * scales.Pixel
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
	photo.Original.Path = path
	// dir, filename := filepath.Split(path)
	// photo.Dir = dir
	// photo.Filename = filename

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
	// photo.bitmaps = append(photo.bitmaps, xl)

	// photo.bitmaps = append(photo.bitmaps, photo.original)
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
			sprite.size.Width/size.Width,
			sprite.size.Height/size.Height,
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
	var bestSize Size
	originalSize := photo.Original.Sprite.size
	bestZoomDist := photo.Original.Sprite.GetPixelZoomDistThumb(c, originalSize)
	for i := range config.Thumbnails {
		thumbnail := &config.Thumbnails[i]
		// thumbSize := thumbnail.Size
		// originalPortrait := originalSize.Width < originalSize.Height
		// thumbPortrait := thumbSize.Width < thumbSize.Height
		// if originalPortrait != thumbPortrait {
		// 	thumbSize.Width, thumbSize.Height = thumbSize.Height, thumbSize.Width
		// }
		thumbSize := thumbnail.Fit(originalSize)
		zoomDist := photo.Original.Sprite.GetPixelZoomDistThumb(c, thumbSize)
		if zoomDist < bestZoomDist {
			best = thumbnail
			bestSize = thumbSize
			bestZoomDist = zoomDist
		}
	}
	if best == nil {
		return &photo.Original
	}

	path, err := best.GetPath(photo.Original.Path)
	if err != nil {
		panic(err)
	}
	scale := photo.Original.Sprite.size.Width / bestSize.Width

	return &Bitmap{
		Path: path,
		Sprite: Sprite{
			size: Size{
				Width:  bestSize.Width,
				Height: bestSize.Height,
			},
			transform: Transform{
				view: photo.Original.Sprite.transform.view.Scale(scale, scale),
			},
		},
	}
}

func (photo *Photo) Draw(config *Config, scene *Scene, c *canvas.Context, scales Scales, source *storage.ImageSource) {
	// photo.original.Draw(c)

	// c.Push()

	// if photo.solid.IsVisible(c, scales) {
	// c.SetFillColor(canvas.Black)
	// 	println("visible")
	// } else {
	// 	c.SetFillColor(canvas.Red)
	// 	println("invisible")
	// }

	if photo.Original.Sprite.IsVisible(c, scales) {

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
			bitmap.Draw(scene, c, scales, source)
			// bitmap.sprite.DrawDebugOverlay(c, scales)
			// bitmap.sprite.DrawText(c, 0, 0, 100, filepath.Base(bitmap.path))
			// bitmap.sprite.DrawText(c, 0, 0, 100, fmt.Sprintf("%d %.2f", bestIndex, bestZoomDist))
		}
		// photo.original.Draw(c)
		// photo.solid.Draw(c)
	}

	// photo.solid.Draw(c)

	// c.Pop()

	// fmt.Printf("%f\n", photo.solid.GetPixelArea(pixelScale))
}

func (solid *Solid) Draw(c *canvas.Context, scales Scales) {
	if solid.Sprite.IsVisible(c, scales) {
		// c.Push()
		prevFill := c.FillColor
		c.SetFillColor(solid.Color)
		solid.Sprite.Draw(c)
		c.SetFillColor(prevFill)
		// c.Pop()
	}
}

// func (scene *Scene) NormalizeRegions() {
// 	for i := range scene.Regions {
// 		region := &scene.Regions[i]
// 		region.X /= scene.Size.Width
// 		region.Y = 1 + (region.Y / scene.Size.Height)
// 		region.W /= scene.Size.Width
// 		region.H /= scene.Size.Height
// 	}
// }

func (scene *Scene) GetRegions(bounds Bounds) []Region {
	rect := canvas.Rect{
		X: bounds.X * scene.Size.Width,
		Y: bounds.Y * scene.Size.Height,
		W: bounds.W * scene.Size.Width,
		H: bounds.H * scene.Size.Height,
	}
	return scene.RegionSource(rect, scene)
	// for i := range scene.Regions {
	// 	region := &scene.Regions[i]
	// 	region.X /= scene.Size.Width
	// 	region.Y = 1 + (region.Y / scene.Size.Height)
	// 	region.W /= scene.Size.Width
	// 	region.H /= scene.Size.Height
	// }
}
