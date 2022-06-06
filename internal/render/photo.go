package render

import (
	"fmt"
	"math"
	"photofield/internal/image"
	"sort"

	"github.com/tdewolff/canvas"
)

type Photo struct {
	Id     image.ImageId
	Sprite Sprite
}

type Variant struct {
	Thumbnail   *image.Thumbnail
	Orientation image.Orientation
	ZoomDist    float64
}

func (variant Variant) String() string {
	name := "original"
	if variant.Thumbnail != nil {
		name = variant.Thumbnail.Name
	}
	return fmt.Sprintf("%0.2f %v", variant.ZoomDist, name)
}

func (photo *Photo) GetSize(source *image.Source) image.Size {
	info := source.GetInfo(photo.Id)
	return image.Size{X: info.Width, Y: info.Height}
}

func (photo *Photo) GetInfo(source *image.Source) image.Info {
	return source.GetInfo(photo.Id)
}

func (photo *Photo) GetPath(source *image.Source) string {
	path, err := source.GetImagePath(photo.Id)
	if err != nil {
		panic("Unable to get photo path")
	}
	return path
}

func (photo *Photo) Place(x float64, y float64, width float64, height float64, source *image.Source) {
	imageSize := photo.GetSize(source)
	imageWidth := float64(imageSize.X)
	imageHeight := float64(imageSize.Y)

	photo.Sprite.PlaceFit(x, y, width, height, imageWidth, imageHeight)
}

func (photo *Photo) getBestBitmaps(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source) []BitmapAtZoom {

	originalInfo := photo.GetInfo(source)
	originalSize := originalInfo.Size()
	originalPath := photo.GetPath(source)
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}

	bitmaps := make([]BitmapAtZoom, 1+len(source.Images.Thumbnails))
	bitmaps[0] = BitmapAtZoom{
		Bitmap: Bitmap{
			Path:        originalPath,
			Orientation: originalInfo.Orientation,
			Sprite:      photo.Sprite,
		},
		ZoomDist: originalZoomDist,
	}

	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
		thumbSize := thumbnail.Fit(originalSize)
		thumbPath := thumbnail.GetPath(originalPath)
		bitmaps[1+i] = BitmapAtZoom{
			Bitmap: Bitmap{
				Path: thumbPath,
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

func (photo *Photo) getBestVariants(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source, originalPath string) []Variant {

	originalInfo := photo.GetInfo(source)
	originalSize := originalInfo.Size()
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}

	variants := make([]Variant, 1+len(source.Images.Thumbnails))
	variants[0] = Variant{
		Thumbnail:   nil,
		Orientation: originalInfo.Orientation,
		ZoomDist:    originalZoomDist,
	}

	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
		thumbSize := thumbnail.Fit(originalSize)
		variants[1+i] = Variant{
			Thumbnail: thumbnail,
			ZoomDist:  photo.Sprite.Rect.GetPixelZoomDist(c, thumbSize) + float64(thumbnail.ExtraCost),
		}
	}

	sort.Slice(variants, func(i, j int) bool {
		a := variants[i]
		b := variants[j]
		return a.ZoomDist < b.ZoomDist
	})

	return variants
}

func (photo *Photo) Draw(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source) {

	pixelArea := photo.Sprite.Rect.GetPixelArea(c, image.Size{X: 1, Y: 1})
	path := photo.GetPath(source)
	if pixelArea < config.MaxSolidPixelArea {
		style := c.Style

		info := source.GetInfo(photo.Id)
		style.FillColor = info.GetColor()

		photo.Sprite.DrawWithStyle(c, style)
		return
	}

	drawn := false
	variants := photo.getBestVariants(config, scene, c, scales, source, path)
	for _, variant := range variants {
		// text := fmt.Sprintf("index %d zd %4.2f %s", index, bitmapAtZoom.ZoomDist, bitmap.Path)
		// println(text)

		bitmap := Bitmap{
			Sprite:      photo.Sprite,
			Orientation: variant.Orientation,
		}

		img, _, err := source.GetImageOrThumbnail(path, variant.Thumbnail)
		if err != nil {
			continue
		}

		if variant.Thumbnail != nil {
			bounds := img.Bounds()
			imgWidth := float64(bounds.Max.X - bounds.Min.X)
			imgHeight := float64(bounds.Max.Y - bounds.Min.Y)
			imgAspect := imgWidth / imgHeight
			imgAspectRotated := 1 / imgAspect
			rectAspect := bitmap.Sprite.Rect.W / bitmap.Sprite.Rect.H
			// In case the image dimensions don't match expected aspect ratio,
			// assume a 90 CCW rotation
			if math.Abs(rectAspect-imgAspect) > math.Abs(rectAspect-imgAspectRotated) {
				bitmap.Orientation = image.Rotate90
			}
		}

		bitmap.DrawImage(config.CanvasImage, img, c)
		drawn = true

		if source.IsSupportedVideo(path) {
			bitmap.DrawVideoIcon(c)
		}

		if config.DebugOverdraw {
			bounds := img.Bounds()
			bitmap.DrawOverdraw(c, bounds.Size())
		}

		if config.DebugThumbnails {
			bounds := img.Bounds()
			text := fmt.Sprintf("%dx%d %s", bounds.Size().X, bounds.Size().Y, variant.String())
			font := scene.Fonts.Debug
			font.Color = canvas.Lime
			bitmap.Sprite.DrawText(c, scales, &font, text)
		}

		break

		// bitmap.Sprite.DrawText(c, scales, &scene.Fonts.Debug, text)
	}

	if !drawn {
		style := c.Style
		style.FillColor = canvas.Red
		photo.Sprite.DrawWithStyle(c, style)
	}

}
