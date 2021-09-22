package layout

import (
	"log"
	"math"
	"photofield/internal/image"
	"photofield/internal/render"
	"time"
)

func LayoutSquare(scene *render.Scene, source *image.Source) {

	// imageWidth := 120.
	photoCount := len(scene.Photos)

	imageWidth := 100.
	imageHeight := imageWidth * 2 / 3

	edgeCount := int(math.Sqrt(float64(photoCount)))

	margin := 1.

	cols := edgeCount
	rows := int(math.Ceil(float64(photoCount) / float64(cols)))

	scene.Bounds = render.Rect{
		X: 0,
		Y: 0,
		W: float64(cols+2) * (imageWidth + margin),
		H: math.Ceil(float64(rows+2)) * (imageHeight + margin),
	}

	// cols := int(scene.size.width/(imageWidth+margin)) - 2

	log.Println("layout")
	lastLogTime := time.Now()
	for i := range scene.Photos {
		photo := &scene.Photos[i]
		col := i % cols
		row := i / cols
		photo.Place((imageWidth+margin)*float64(1+col), (imageHeight+margin)*float64(1+row), imageWidth, imageHeight, source)
		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, photoCount)
		}
	}

}
