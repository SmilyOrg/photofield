package layout

import (
	// . "photofield/internal"

	"log"
	"photofield/internal/collection"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"

	"time"
)

func LayoutStrip(layout Layout, collection collection.Collection, scene *render.Scene, source *image.Source) {

	limit := collection.Limit

	var infos <-chan image.SourcedInfo

	if scene.Search != "" {
		infos = image.SimilarityInfosToSourcedInfos(
			collection.GetSimilar(source, scene.SearchEmbedding, image.ListOptions{
				Limit: limit,
			}),
		)
	} else {
		infos = collection.GetInfos(source, image.ListOptions{
			OrderBy: image.ListOrder(layout.Order),
			Limit:   limit,
		})
	}

	layout.ImageSpacing = 0.02 * layout.ViewportWidth

	rect := render.Rect{
		X: 0,
		Y: 0,
		W: layout.ViewportWidth,
		H: layout.ViewportHeight,
	}

	scene.Bounds.H = float64(rect.H)

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	layoutPlaced := metrics.Elapsed("layout placing")
	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	lastLogTime := time.Now()

	scene.Photos = scene.Photos[:0]
	index := 0
	for info := range infos {
		if limit > 0 && index >= limit {
			break
		}

		imageRect := render.Rect{
			X: 0,
			Y: 0,
			W: float64(info.Width),
			H: float64(info.Height),
		}

		scene.Photos = append(scene.Photos, render.Photo{
			Id: info.Id,
			Sprite: render.Sprite{
				Rect: imageRect.FitInside(rect),
			},
		})

		rect.X += float64(rect.W) + layout.ImageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout strip %d\n", index)
		}

		layoutCounter.Set(index)
		index++
		scene.FileCount = index
	}
	layoutPlaced()

	scene.Bounds.W = rect.X

	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}
}
