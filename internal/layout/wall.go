package photofield

import (
	"fmt"
	"math"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
)

func LayoutWall(config Layout, scene *Scene, source *storage.ImageSource) {

	photoCount := len(scene.Photos)

	edgeCount := int(math.Sqrt(float64(photoCount)))

	scene.Bounds.W = config.SceneWidth
	cols := edgeCount

	layoutConfig := Layout{}
	layoutConfig.ImageSpacing = config.SceneWidth / float64(edgeCount) * 0.02
	layoutConfig.LineSpacing = layoutConfig.ImageSpacing

	fmt.Printf("scene width %v cols %v\n", scene.Bounds.W, cols)

	imageWidth := scene.Bounds.W / (float64(cols) - layoutConfig.ImageSpacing)
	imageHeight := imageWidth * 2 / 3 * 1.2

	fmt.Printf("image %f %f\n", imageWidth, imageHeight)

	rows := int(math.Ceil(float64(photoCount) / float64(cols)))

	scene.Bounds.H = math.Ceil(float64(rows)) * (imageHeight + layoutConfig.LineSpacing)

	fmt.Printf("scene %f %f\n", scene.Bounds.W, scene.Bounds.H)

	sceneMargin := 10.
	layoutConfig.ImageHeight = imageHeight

	section := Section{}
	for i := range scene.Photos {
		photo := &scene.Photos[i]
		section.photos = append(section.photos, photo)
	}

	x := sceneMargin
	y := sceneMargin

	photos := make(chan SectionPhoto, 1)
	boundsOut := make(chan Rect)
	go layoutSectionPhotos(photos, Rect{
		X: x,
		Y: y,
		W: scene.Bounds.W - sceneMargin*2,
		H: scene.Bounds.H - sceneMargin*2,
	}, boundsOut, layoutConfig, scene, source)
	go getSectionPhotos(&section, photos, source)

	newBounds := <-boundsOut

	scene.Bounds.H = newBounds.Y + newBounds.H + sceneMargin
}
