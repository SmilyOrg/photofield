package render

import (
	"context"
	"fmt"
	"log"
	"math"
	"photofield/internal/image"
	"photofield/io"
	"sort"
	"time"

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

func (photo *Photo) getBestVariants(config *Render, scene *Scene, c *canvas.Context, scales Scales, source *image.Source, originalPath string) []Variant {

	originalInfo := photo.GetInfo(source)
	originalSize := originalInfo.Size()
	originalZoomDist := math.Inf(1)
	if source.IsSupportedImage(originalPath) {
		originalZoomDist = photo.Sprite.Rect.GetPixelZoomDist(c, originalSize)
	}

	thumbnails := source.GetApplicableThumbnails(originalPath)
	variants := make([]Variant, 1+len(thumbnails))
	variants[0] = Variant{
		Thumbnail:   nil,
		Orientation: originalInfo.Orientation,
		ZoomDist:    originalZoomDist,
	}

	for i := range thumbnails {
		thumbnail := &thumbnails[i]
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

	// rect := photo.Sprite.Rect
	info := source.GetInfo(photo.Id)
	size := info.Size()
	rsize := photo.Sprite.Rect.RenderedSize(c, size)

	// ids := ristretto.IdWithSize{
	// 	Id:   io.ImageId(photo.Id),
	// 	Size: io.Size(rsize),
	// }

	// r := source.Ristretto.GetWithSize(context.TODO(), ids)
	// if r.Image != nil {
	// 	fmt.Printf("%s %4d %4d %s %d\n", ids.String(), r.Image.Bounds().Dx(), r.Image.Bounds().Dy(), r.Error, r.Orientation)

	// 	bitmap := Bitmap{
	// 		Sprite:      photo.Sprite,
	// 		Orientation: image.Orientation(r.Orientation),
	// 	}

	// 	img := r.Image
	// 	bitmap.DrawImage(config.CanvasImage, img, c)
	// 	drawn = true

	// 	if source.IsSupportedVideo(path) {
	// 		bitmap.DrawVideoIcon(c)
	// 	}

	// 	if config.DebugOverdraw {
	// 		bounds := img.Bounds()
	// 		bitmap.DrawOverdraw(c, bounds.Size())
	// 	}

	// 	if config.DebugThumbnails {
	// 		size := img.Bounds().Size()
	// 		text := fmt.Sprintf("%dx%d", size.X, size.Y)
	// 		font := scene.Fonts.Debug
	// 		font.Color = canvas.Lime
	// 		bitmap.Sprite.DrawText(config, c, scales, &font, text)
	// 	}
	// }

	sources := source.Sources.EstimateCost(io.Size(size), io.Size(rsize))
	sources.Sort()
	for i, s := range sources {
		if drawn {
			break
		}
		start := time.Now()
		r := s.Get(context.TODO(), io.ImageId(photo.Id), path)
		elapsed := time.Since(start).Microseconds()

		img, err := r.Image, r.Error
		if r.Orientation == io.SourceInfoOrientation {
			r.Orientation = io.Orientation(info.Orientation)
		}

		// imgb := "0x0"
		// if img != nil {
		// 	b := img.Bounds()
		// 	imgb = fmt.Sprintf("%d x %d", b.Dx(), b.Dy())
		// }
		// fmt.Printf("%4d %6f %s %s %s\n", i, s.Cost, s.Name(), imgb, err)

		if img == nil || err != nil {
			continue
		}

		// for j := i; j >= 0; j-- {
		// 	ss := sources[j]
		// 	ret := ss.Set(context.TODO(), io.ImageId(photo.Id), path, img, err)
		// 	imgb := "0x0"
		// 	if img != nil {
		// 		b := img.Bounds()
		// 		imgb = fmt.Sprintf("%d x %d", b.Dx(), b.Dy())
		// 	}
		// 	fmt.Printf("set %4d %s %s %s %v\n", j, ss.Name(), imgb, err, ret)
		// }

		source.SourcesLatencyHistogram.WithLabelValues(s.Name()).Observe(float64(elapsed))

		// savedstr := "saved"
		// source.Ristretto.SetWithSize(context.TODO(), ids, r)
		// fmt.Printf("%4d %6d us %s %s %s\n", i, elapsed, s.Name(), imgb, err)
		// fmt.Printf("%4d %6d us %s\n", i, elapsed, s.Name())

		// if !saved {
		// 	savedstr = "not saved"
		// }

		// fmt.Printf("%4d %6d us %s %s %s %s\n", i, elapsed, s.Name(), imgb, err, savedstr)

		// fmt.Printf("%4d %6.2f %6d us %s %s %s %d\n", i, s.Cost, elapsed, s.Name(), imgb, err, r.Orientation)
		// fmt.Printf("%4d %4d %4d %4d %d\n", info.Width, info.Height, img.Bounds().Dx(), img.Bounds().Dy(), r.Orientation)
		// fmt.Printf("%4d %4d %4d %4d %d\n", info.Width, info.Height, img.Bounds().Dx(), img.Bounds().Dy(), r.Orientation)

		bitmap := Bitmap{
			Sprite:      photo.Sprite,
			Orientation: image.Orientation(r.Orientation),
		}
		// if s.Rotate() {
		// 	bitmap.Orientation = info.Orientation
		// }

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

	// variants := photo.getBestVariants(config, scene, c, scales, source, path)
	// for _, variant := range variants {
	// 	break
	// 	// text := fmt.Sprintf("index %d zd %4.2f %s", index, bitmapAtZoom.ZoomDist, bitmap.Path)
	// 	// println(text)

	// 	bitmap := Bitmap{
	// 		Sprite:      photo.Sprite,
	// 		Orientation: variant.Orientation,
	// 	}

	// 	img, _, err := source.GetImageOrThumbnail(path, variant.Thumbnail)
	// 	if err != nil {
	// 		continue
	// 	}

	// 	if variant.Thumbnail != nil {
	// 		bounds := img.Bounds()
	// 		imgWidth := float64(bounds.Max.X - bounds.Min.X)
	// 		imgHeight := float64(bounds.Max.Y - bounds.Min.Y)
	// 		imgAspect := imgWidth / imgHeight
	// 		imgAspectRotated := 1 / imgAspect
	// 		rectAspect := bitmap.Sprite.Rect.W / bitmap.Sprite.Rect.H
	// 		// In case the image dimensions don't match expected aspect ratio,
	// 		// assume a 90 CCW rotation
	// 		if math.Abs(rectAspect-imgAspect) > math.Abs(rectAspect-imgAspectRotated) {
	// 			bitmap.Orientation = image.Rotate90
	// 		}
	// 	}

	// 	bitmap.DrawImage(config.CanvasImage, img, c)
	// 	drawn = true

	// 	if source.IsSupportedVideo(path) {
	// 		bitmap.DrawVideoIcon(c)
	// 	}

	// 	if config.DebugOverdraw {
	// 		bounds := img.Bounds()
	// 		bitmap.DrawOverdraw(c, bounds.Size())
	// 	}

	// 	if config.DebugThumbnails {
	// 		bounds := img.Bounds()
	// 		text := fmt.Sprintf("%dx%d %s", bounds.Size().X, bounds.Size().Y, variant.String())
	// 		font := scene.Fonts.Debug
	// 		font.Color = canvas.Lime
	// 		bitmap.Sprite.DrawText(config, c, scales, &font, text)
	// 	}

	// 	break

	// 	// bitmap.Sprite.DrawText(c, scales, &scene.Fonts.Debug, text)
	// }

	if !drawn {
		style := c.Style
		style.FillColor = canvas.Red
		photo.Sprite.DrawWithStyle(c, style)
	}

}
