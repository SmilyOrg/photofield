package layout

import (
	// . "photofield/internal"

	"log"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"

	"time"

	"github.com/tdewolff/canvas"
)

type AlbumEvent struct {
	StartTime  time.Time
	EndTime    time.Time
	First      bool
	FirstOnDay bool
	LastOnDay  bool
	Elapsed    time.Duration
	Section    Section
	Location   string
}

func LayoutAlbumEvent(layout Layout, rect render.Rect, event *AlbumEvent, scene *render.Scene, source *image.Source) render.Rect {

	if event.FirstOnDay {
		dateFormat := "Monday, Jan 2"
		if event.First {
			dateFormat = "Monday, Jan 2, 2006"
		}
		text := render.NewTextFromRect(
			render.Rect{
				X: rect.X,
				Y: rect.Y,
				W: rect.W,
				H: 40,
			},
			&scene.Fonts.Header,
			event.StartTime.Format(dateFormat),
		)
		text.VAlign = canvas.Bottom
		scene.Texts = append(scene.Texts, text)
		rect.Y += text.Sprite.Rect.H
	}

	font := scene.Fonts.Main.Face(50, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	time := event.StartTime.Format("15:00")
	text := render.NewTextFromRect(
		render.Rect{
			X: rect.X,
			Y: rect.Y,
			W: rect.W,
			H: 40,
		},
		&font,
		time,
	)
	text.VAlign = canvas.Bottom
	scene.Texts = append(scene.Texts, text)
	rect.Y += text.Sprite.Rect.H

	newBounds := addSectionToScene(&event.Section, scene, rect, layout, source)

	rect.Y = newBounds.Y + newBounds.H
	if event.LastOnDay {
		rect.Y += 32
	}
	return rect
}

func LayoutAlbum(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.

	scene.Bounds.W = layout.ViewportWidth

	event := AlbumEvent{
		First: true,
	}
	eventCount := 0
	var lastPhotoTime time.Time

	rect := render.Rect{
		X: sceneMargin,
		Y: sceneMargin + 64,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	layoutPlaced := metrics.Elapsed("layout placing")
	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	scene.Photos = scene.Photos[:0]
	index := 0
	for info := range infos {
		photoTime := info.DateTime
		elapsed := photoTime.Sub(lastPhotoTime)
		if elapsed > 1*time.Hour {
			if eventCount > 0 {
				event.EndTime = lastPhotoTime
				event.LastOnDay = !SameDay(lastPhotoTime, photoTime)
				rect = LayoutAlbumEvent(layout, rect, &event, scene, source)
				scene.Bounds.H = rect.Y
			}
			eventCount++
			event = AlbumEvent{
				First:      eventCount == 1,
				StartTime:  photoTime,
				FirstOnDay: !SameDay(lastPhotoTime, photoTime),
				Elapsed:    elapsed,
				Section: Section{
					infos: event.Section.infos[:0],
				},
			}
		}
		lastPhotoTime = photoTime

		event.Section.infos = append(event.Section.infos, info)

		// time.Sleep(1 * time.Millisecond)

		layoutCounter.Set(index)
		index++
		scene.FileCount = index
	}
	layoutPlaced()

	if len(event.Section.infos) > 0 {
		event.EndTime = lastPhotoTime
		event.LastOnDay = true
		rect = LayoutAlbumEvent(layout, rect, &event, scene, source)
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	// scene.Bounds.H = rect.Y*0.5 + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}

}
