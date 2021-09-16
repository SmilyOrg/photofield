package photofield

import (
	// . "photofield/internal"

	"log"

	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
	"time"

	"github.com/tdewolff/canvas"
)

type AlbumEvent struct {
	StartTime  time.Time
	EndTime    time.Time
	First      bool
	FirstOnDay bool
	LastOnDay  bool
	Section    Section
}

func LayoutAlbumEvent(layout Layout, rect Rect, event *AlbumEvent, scene *Scene, source *storage.ImageSource) Rect {

	if event.FirstOnDay {
		font := layout.FontFamily.Face(70, canvas.Black, canvas.FontRegular, canvas.FontNormal)
		dateFormat := "Monday, Jan 2"
		if event.First {
			dateFormat = "Monday, Jan 2, 2006"
		}
		text := NewTextFromRect(
			Rect{
				X: rect.X,
				Y: rect.Y,
				W: rect.W,
				H: 30,
			},
			&font,
			event.StartTime.Format(dateFormat),
		)
		scene.Texts = append(scene.Texts, text)
		rect.Y += text.Sprite.Rect.H + 15
	}

	font := layout.FontFamily.Face(50, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	text := NewTextFromRect(
		Rect{
			X: rect.X,
			Y: rect.Y,
			W: rect.W,
			H: 30,
		},
		&font,
		event.StartTime.Format("15:00"),
	)
	scene.Texts = append(scene.Texts, text)
	rect.Y += text.Sprite.Rect.H + 10

	photos := addSectionPhotos(&event.Section, scene, source)
	newBounds := layoutSectionPhotos(photos, rect, layout, scene, source)

	rect.Y = newBounds.Y + newBounds.H
	if event.LastOnDay {
		rect.Y += 40
	} else {
		rect.Y += 6
	}
	return rect
}

func LayoutAlbum(layout Layout, collection Collection, scene *Scene, source *storage.ImageSource) {

	limit := collection.Limit

	infos := collection.GetInfos(source, ListOptions{
		OrderBy: DateAsc,
		Limit:   limit,
	})

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.

	scene.Bounds.W = layout.SceneWidth

	event := AlbumEvent{
		First: true,
	}
	eventCount := 0
	var lastPhotoTime time.Time

	rect := Rect{
		X: sceneMargin,
		Y: sceneMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	if layout.FontFamily == nil {
		layout.FontFamily = canvas.NewFontFamily("Roboto")
		err := layout.FontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
		if err != nil {
			panic(err)
		}
		err = layout.FontFamily.LoadFontFile("fonts/Roboto/Roboto-Bold.ttf", canvas.FontBold)
		if err != nil {
			panic(err)
		}
	}
	if layout.HeaderFont == nil {
		face := layout.FontFamily.Face(80.0, canvas.Gray, canvas.FontRegular, canvas.FontNormal)
		layout.HeaderFont = &face
	}

	scene.Solids = make([]Solid, 0)
	scene.Texts = make([]Text, 0)

	layoutPlaced := Elapsed("layout placing")
	layoutCounter := Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	scene.Photos = scene.Photos[:0]
	index := 0
	for info := range infos {
		if limit > 0 && index >= limit {
			break
		}

		photoTime := info.DateTime
		elapsed := photoTime.Sub(lastPhotoTime)
		if elapsed > 1*time.Hour {
			if eventCount > 0 {
				event.EndTime = lastPhotoTime
				event.LastOnDay = !SameDay(lastPhotoTime, photoTime)
				rect = LayoutAlbumEvent(layout, rect, &event, scene, source)
			}
			eventCount++
			event = AlbumEvent{
				First:      eventCount == 1,
				StartTime:  photoTime,
				FirstOnDay: !SameDay(lastPhotoTime, photoTime),
			}
		}
		lastPhotoTime = photoTime

		event.Section.infos = append(event.Section.infos, info)

		layoutCounter.Set(index)
		index++
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
	scene.RegionSource = PhotoRegionSource{
		imageSource: source,
	}

}
