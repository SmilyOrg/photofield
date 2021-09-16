package photofield

import (
	"log"
	"math"
	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
	"time"
)

func LayoutWall(layout Layout, collection Collection, scene *Scene, source *storage.ImageSource) {

	infos := collection.GetInfos(source, ListOptions{
		OrderBy: DateAsc,
		Limit:   collection.Limit,
	})

	section := Section{}

	loadCounter := Counter{
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

	scene.Bounds.W = layout.SceneWidth
	cols := edgeCount

	layoutConfig := Layout{}
	layoutConfig.ImageSpacing = layout.SceneWidth / float64(edgeCount) * 0.02
	layoutConfig.LineSpacing = layoutConfig.ImageSpacing

	log.Printf("layout wall width %v cols %v\n", scene.Bounds.W, cols)

	imageWidth := scene.Bounds.W / (float64(cols) - layoutConfig.ImageSpacing)
	imageHeight := imageWidth * 2 / 3 * 1.2

	log.Printf("layout wall image %f %f\n", imageWidth, imageHeight)

	rows := int(math.Ceil(float64(photoCount) / float64(cols)))

	scene.Bounds.H = math.Ceil(float64(rows)) * (imageHeight + layoutConfig.LineSpacing)

	sceneMargin := 10.
	layoutConfig.ImageHeight = imageHeight

	x := sceneMargin
	y := sceneMargin

	layoutFinished := Elapsed("layout")
	photos := addSectionPhotos(&section, scene, source)
	newBounds := layoutSectionPhotos(photos, Rect{
		X: x,
		Y: y,
		W: scene.Bounds.W - sceneMargin*2,
		H: scene.Bounds.H - sceneMargin*2,
	}, layoutConfig, scene, source)
	layoutFinished()

	scene.Bounds.H = newBounds.Y + newBounds.H + sceneMargin
}
