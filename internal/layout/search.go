package layout

import (
	"log"
	"math"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"

	"time"
)

func LayoutSearch(infos <-chan image.SimilarityInfo, layout Layout, scene *render.Scene, source *image.Source) {

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.
	falloff := 5.

	scene.Bounds.W = layout.ViewportWidth

	rect := render.Rect{
		X: sceneMargin,
		Y: sceneMargin + 60,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	layoutDone := metrics.Elapsed("layout placing")
	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	scene.Photos = scene.Photos[:0]
	rowIndex := 0
	index := 0
	lastLogTime := time.Now()
	mostSimilar := float32(0)
	imageHeight := layout.ImageHeight

	for info := range infos {
		photo := render.Photo{
			Id:     info.Id,
			Sprite: render.Sprite{},
		}

		if index == 0 {
			mostSimilar = info.Similarity
		}

		aspectRatio := float64(info.Width) / float64(info.Height)
		imageWidth := float64(imageHeight) * aspectRatio

		if rect.X+imageWidth > rect.W {
			scale := layoutFitRow(scene.Photos[rowIndex:], rect, layout.ImageSpacing)
			rowIndex = len(scene.Photos)
			rect.X = sceneMargin
			rect.Y += imageHeight*scale + layout.LineSpacing

			nsim := info.Similarity / mostSimilar
			pnsim := math.Pow(float64(nsim), falloff)
			imageHeight = layout.ImageHeight * pnsim
			imageWidth = float64(imageHeight) * aspectRatio
		}

		photo.Sprite.PlaceFitHeight(
			rect.X,
			rect.Y,
			imageHeight,
			float64(info.Width),
			float64(info.Height),
		)

		scene.Photos = append(scene.Photos, photo)

		rect.X += imageWidth + layout.ImageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout %d\n", index)
		}

		layoutCounter.Set(index)
		index++
		scene.FileCount = index
	}
	layoutDone()

	rect.X = sceneMargin
	rect.Y += layout.ImageHeight + layout.LineSpacing

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}

}
