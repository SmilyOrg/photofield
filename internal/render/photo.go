package render

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"math"
	"photofield/internal/image"
	"photofield/io"
	"runtime/trace"
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

func (photo *Photo) Draw(ctx context.Context, config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source, selected bool) {
	defer trace.StartRegion(ctx, "photo.Draw").End()

	pixelArea := photo.Sprite.Rect.GetPixelArea(c, image.Size{X: 1, Y: 1})

	// Avoid drawing almost-invisible photos or squares
	if pixelArea < 0.1 {
		return
	}

	if config.TransparencyMask {
		style := c.Style
		style.FillColor = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
		photo.Sprite.DrawWithStyle(c, style)
		return
	}

	if pixelArea < config.MaxSolidPixelArea {
		style := c.Style

		scale := 1.
		if selected {
			style := c.Style
			style.FillColor = color.RGBA{0xe4, 0xf2, 0xff, 0xff}
			photo.Sprite.DrawWithStyle(c, style)
			scale = 0.8
		}

		// TODO: this can be a bottleneck for lots of images
		// if it ends up hitting the database for each individual image
		info := source.GetInfo(photo.Id)
		style.FillColor = info.GetColor()

		photo.Sprite.DrawInsetWithStyle(c, style, (1-scale)*photo.Sprite.Rect.W)
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

	hq := false
	if config.QualityPreset == QualityPresetHigh {
		hq = true
	}
	costOpts := io.DefaultOptions
	if hq {
		costOpts.UnderdrawPenaltyMultiplier = 1000
		costOpts.DurationCostMultiplier = 0
	}
	sources := srcs.EstimateCostWithOpts(io.Size(size), io.Size(rsize), costOpts)
	sources.Sort()

	var errs []error

	for i, s := range sources {
		if drawn {
			break
		}
		start := time.Now()
		r := s.Get(ctx, io.ImageId(photo.Id), path, io.Size(size))
		elapsed := time.Since(start)

		img, err := r.Image, r.Error
		if img == nil || err != nil {
			if err != nil {
				errs = append(errs, err)
			}
			continue
		}

		if !r.FromCache {
			name := s.Name()
			elapsedus := float64(elapsed.Microseconds())
			elapsedabsdiff := math.Abs(float64(s.EstimatedDuration.Microseconds()) - elapsedus)
			source.SourceLatencyHistogram.WithLabelValues(name).Observe(elapsedus)
			source.SourceLatencyAbsDiffHistogram.WithLabelValues(name).Observe(elapsedabsdiff)
			source.SourcePerOriginalMegapixelLatencyHistogram.WithLabelValues(name).Observe(elapsedus * 1e6 / (float64(size.X) * float64(size.Y)))
			source.SourcePerResizedMegapixelLatencyHistogram.WithLabelValues(name).Observe(elapsedus * 1e6 / float64(s.EstimatedArea))
		}

		if r.Orientation == io.SourceInfoOrientation {
			r.Orientation = io.Orientation(info.Orientation)
		}

		bitmap := Bitmap{
			Sprite:      photo.Sprite,
			Orientation: image.Orientation(r.Orientation),
		}

		scale := 1.
		if selected {
			style := c.Style
			style.FillColor = color.RGBA{0xe4, 0xf2, 0xff, 0xff}
			bitmap.Sprite.DrawWithStyle(c, style)
			scale = 0.8
		}

		bitmap.DrawImage(ctx, config.CanvasImage, img, c, scale, hq)
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
			text := fmt.Sprintf("%dx%d %d %4.0f\n%s", size.X, size.Y, i, s.Cost, s.Name())
			font := scene.Fonts.Debug
			font.Color = canvas.Yellow
			s := bitmap.Sprite
			s.DrawText(config, c, scales, &font, text)
		}

		break
	}

	if !drawn {
		if config.DebugThumbnails && len(errs) > 0 {
			log.Printf("Unable to draw photo %v: %v", photo.Id, errs)
		}

		style := c.Style
		style.FillColor = canvas.Red
		photo.Sprite.DrawWithStyle(c, style)
	}

}
