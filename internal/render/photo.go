package render

import (
	"math"
	"photofield/internal/image"
	"sort"

	"github.com/tdewolff/canvas"
)

type Photo struct {
	Id     image.ImageId
	Sprite Sprite
}

func (photo *Photo) GetSize(source *image.Source) image.Size {
	info := source.GetInfo(photo.GetPath(source))
	return image.Size{X: info.Width, Y: info.Height}
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

func (photo *Photo) getBestBitmap(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source) (Bitmap, float64) {
	var best *image.Thumbnail
	originalSize := photo.GetSize(source)
	originalPath := photo.GetPath(source)
	originalOrientation := source.GetOrientation(originalPath)
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}
	// fmt.Printf("%4.0f %4.0f\n", photo.Original.Sprite.Rect.W, photo.Original.Sprite.Rect.H)
	bestZoomDist := originalZoomDist
	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
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
			Path:        originalPath,
			Orientation: originalOrientation,
			Sprite:      photo.Sprite,
		}, originalZoomDist
	}

	bestPath := best.GetPath(originalPath)
	bestOrientation := source.GetOrientation(bestPath)

	return Bitmap{
		Path:        bestPath,
		Orientation: bestOrientation,
		Sprite: Sprite{
			Rect: photo.Sprite.Rect,
		},
	}, bestZoomDist
}

func (photo *Photo) getBestBitmaps(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source) []BitmapAtZoom {

	originalSize := photo.GetSize(source)
	originalPath := photo.GetPath(source)
	originalOrientation := source.GetOrientation(originalPath)
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}

	bitmaps := make([]BitmapAtZoom, 1+len(source.Images.Thumbnails))
	bitmaps[0] = BitmapAtZoom{
		Bitmap: Bitmap{
			Path:        originalPath,
			Orientation: originalOrientation,
			Sprite:      photo.Sprite,
		},
		ZoomDist: originalZoomDist,
	}

	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
		thumbSize := thumbnail.Fit(originalSize)
		thumbPath := thumbnail.GetPath(originalPath)
		thumbOrientation := source.GetOrientation(thumbPath)
		bitmaps[1+i] = BitmapAtZoom{
			Bitmap: Bitmap{
				Path:        thumbPath,
				Orientation: thumbOrientation,
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

func (photo *Photo) Draw(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source) {

	pixelArea := photo.Sprite.Rect.GetPixelArea(c, image.Size{X: 1, Y: 1})
	path := photo.GetPath(source)
	if pixelArea < config.MaxSolidPixelArea {
		style := c.Style

		info := source.GetInfo(path)
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

			if source.IsSupportedVideo(path) {
				bitmap.DrawVideoIcon(c)
			}

			if config.DebugOverdraw {
				bitmap.DrawOverdraw(c, source)
			}

			if config.DebugThumbnails {
				text := ""

				for i := range source.Images.Thumbnails {
					thumbnail := &source.Images.Thumbnails[i]
					thumbnailPath := thumbnail.GetPath(photo.GetPath(source))
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
