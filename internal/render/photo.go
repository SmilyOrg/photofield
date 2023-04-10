package render

import (
	"context"
	"fmt"
	"log"
	"photofield/internal/image"
	"photofield/io"
	"time"

	"github.com/tdewolff/canvas"
)

type Photo struct {
	Id     image.ImageId
	Sprite Sprite
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
		log.Fatalf("Unable to get photo path for id %v", photo.Id)
	}
	return path
}

func (photo *Photo) Place(x float64, y float64, width float64, height float64, source *image.Source) {
	imageSize := photo.GetSize(source)
	imageWidth := float64(imageSize.X)
	imageHeight := float64(imageSize.Y)

	photo.Sprite.PlaceFit(x, y, width, height, imageWidth, imageHeight)
}

func (photo *Photo) Draw(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source) {

	pixelArea := photo.Sprite.Rect.GetPixelArea(c, image.Size{X: 1, Y: 1})
	if pixelArea < config.MaxSolidPixelArea {
		style := c.Style

		// TODO: this can be a bottleneck for lots of images
		// if it ends up hitting the database for each individual image
		info := source.GetInfo(photo.Id)
		style.FillColor = info.GetColor()

		photo.Sprite.DrawWithStyle(c, style)
		return
	}

	drawn := false
	path := photo.GetPath(source)

	info := source.GetInfo(photo.Id)
	size := info.Size()
	rsize := photo.Sprite.Rect.RenderedSize(c, size)

	srcs := source.Sources
	if config.Sources != nil {
		srcs = config.Sources
	}
	sources := srcs.EstimateCost(io.Size(size), io.Size(rsize))
	sources.Sort()
	for i, s := range sources {
		if drawn {
			break
		}
		start := time.Now()
		r := s.Get(context.TODO(), io.ImageId(photo.Id), path)
		elapsed := time.Since(start)

		img, err := r.Image, r.Error
		if img == nil || err != nil {
			continue
		}

		name := s.Name()
		source.SourceLatencyHistogram.WithLabelValues(name).Observe(float64(elapsed.Microseconds()))
		source.SourcePerOriginalMegapixelLatencyHistogram.WithLabelValues(name).Observe(float64(elapsed) * 1e6 / (float64(size.X) * float64(size.Y)))
		source.SourcePerResizedMegapixelLatencyHistogram.WithLabelValues(name).Observe(float64(elapsed) * 1e6 / float64(s.EstimatedArea))

		if r.Orientation == io.SourceInfoOrientation {
			r.Orientation = io.Orientation(info.Orientation)
		}

		bitmap := Bitmap{
			Sprite:      photo.Sprite,
			Orientation: image.Orientation(r.Orientation),
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
			size := img.Bounds().Size()
			text := fmt.Sprintf("%dx%d %d %4f\n%s", size.X, size.Y, i, s.Cost, s.Name())
			font := scene.Fonts.Debug
			font.Color = canvas.Yellow
			s := bitmap.Sprite
			s.Rect.Y -= 20
			s.DrawText(config, c, scales, &font, text)
		}

		break
	}

	if !drawn {
		style := c.Style
		style.FillColor = canvas.Red
		photo.Sprite.DrawWithStyle(c, style)
	}

}
