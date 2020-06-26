package photofield

import (
	"fmt"
	"math"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
)

func LayoutWall(config *RenderConfig, scene *Scene, source *storage.ImageSource) {

	photoCount := len(scene.Photos)

	imageHeight := 100.
	imageWidth := imageHeight * 3 / 2 * 0.8

	edgeCount := int(math.Sqrt(float64(photoCount)))

	margin := 1.

	cols := edgeCount
	rows := int(math.Ceil(float64(photoCount) / float64(cols)))

	scene.Bounds = Rect{
		X: 0,
		Y: 0,
		W: float64(cols+2) * (imageWidth + margin),
		H: math.Ceil(float64(rows+2)) * (imageHeight + margin),
	}

	fmt.Printf("%f %f\n", scene.Bounds.W, scene.Bounds.H)

	imageSpacing := 3.
	lineSpacing := 3.
	sceneMargin := 10.

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
	}, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)

	newBounds := <-boundsOut

	scene.Bounds.H = newBounds.Y + newBounds.H + sceneMargin
}
