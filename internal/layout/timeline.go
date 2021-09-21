package photofield

import (
	"log"
	"time"

	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	storage "photofield/internal/storage"

	"github.com/hako/durafmt"
	"github.com/tdewolff/canvas"
)

type TimelineEvent struct {
	StartTime  time.Time
	EndTime    time.Time
	First      bool
	FirstOnDay bool
	LastOnDay  bool
	Section    Section
}

func LayoutTimelineEvent(layout Layout, rect Rect, event *TimelineEvent, scene *Scene, source *storage.ImageSource) Rect {

	// log.Println("layout event", len(event.Section.photos), rect.X, rect.Y)

	textHeight := 30.
	textBounds := Rect{
		X: rect.X,
		Y: rect.Y,
		W: rect.W,
		H: textHeight,
	}

	startTimeFormat := "Mon, Jan 2"
	if event.StartTime.Year() != time.Now().Year() {
		startTimeFormat += ", 2006"
	}

	startTimeFormat += "   15:04"

	headerText := event.StartTime.Format(startTimeFormat)

	duration := event.EndTime.Sub(event.StartTime)
	if duration >= 1*time.Minute {
		dur := durafmt.Parse(duration)
		headerText += "   " + dur.LimitFirstN(1).String()
	}

	font := scene.Fonts.Main.Face(40, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	scene.Texts = append(scene.Texts,
		NewTextFromRect(
			textBounds,
			&font,
			headerText,
		),
	)
	rect.Y += textHeight + 15

	photos := addSectionPhotos(&event.Section, scene, source)
	newBounds := layoutSectionPhotos(photos, rect, layout, scene, source)

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += 10
	return rect
}

func LayoutTimeline(layout Layout, collection Collection, scene *Scene, source *storage.ImageSource) {

	limit := collection.Limit

	infos := collection.GetInfos(source, ListOptions{
		OrderBy: DateDesc,
		Limit:   limit,
	})

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.

	scene.Bounds.W = layout.SceneWidth

	event := TimelineEvent{}
	eventCount := 0
	var lastPhotoTime time.Time

	rect := Rect{
		X: sceneMargin,
		Y: sceneMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	scene.Solids = make([]Solid, 0)
	scene.Texts = make([]Text, 0)

	layoutPlaced := Elapsed("layout placing")
	layoutCounter := Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	index := 0
	for info := range infos {
		if limit > 0 && index >= limit {
			break
		}

		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 30*time.Minute {
			event.StartTime = lastPhotoTime
			rect = LayoutTimelineEvent(layout, rect, &event, scene, source)
			eventCount++
			event = TimelineEvent{
				EndTime: photoTime,
			}
		}
		lastPhotoTime = photoTime

		event.Section.infos = append(event.Section.infos, info)

		layoutCounter.Set(index)
		index++
	}
	layoutPlaced()

	if len(event.Section.infos) > 0 {
		event.StartTime = lastPhotoTime
		rect = LayoutTimelineEvent(layout, rect, &event, scene, source)
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		imageSource: source,
	}

}
