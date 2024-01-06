package layout

import (
	"log"
	"math"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"time"
)

func LayoutWall(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	section := Section{}

	loadCounter := metrics.Counter{
		Name:     "load infos",
		Interval: 1 * time.Second,
	}

	index := 0
	for info := range infos {
		section.infos = append(section.infos, info)
		loadCounter.Set(index)
		index++
	}

	photoCount := len(section.infos)

	edgeCount := int(math.Sqrt(float64(photoCount)))
	if edgeCount < 1 {
		edgeCount = 1
	}

	sceneMargin := 10.
	scene.Bounds.W = layout.ViewportWidth
	cols := edgeCount

	bounds := render.Rect{
		X: sceneMargin,
		Y: sceneMargin + 64,
		W: scene.Bounds.W - sceneMargin*2,
		H: scene.Bounds.H - sceneMargin*2,
	}

	layoutConfig := Layout{}
	layoutConfig.ImageSpacing = bounds.W / float64(cols) * 0.02
	layoutConfig.LineSpacing = layoutConfig.ImageSpacing
	imageWidth := bounds.W / float64(cols)

	log.Printf("layout wall width %v cols %v\n", scene.Bounds.W, cols)

	imageHeight := imageWidth * 2 / 3 * 1.2

	log.Printf("layout wall image %f %f\n", imageWidth, imageHeight)

	layoutConfig.ImageHeight = imageHeight

	layoutFinished := metrics.Elapsed("layout")
	newBounds := addSectionToScene(&section, scene, bounds, layoutConfig, source)
	layoutFinished()

	scene.Bounds.H = newBounds.Y + newBounds.H + sceneMargin
}
