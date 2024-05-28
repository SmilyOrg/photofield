package layout

import (
	"context"
	"log"
	"time"

	"github.com/golang/geo/s2"
	"github.com/hako/durafmt"
	"github.com/tdewolff/canvas"

	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
)

type TimelineEvent struct {
	StartTime  time.Time
	EndTime    time.Time
	First      bool
	FirstOnDay bool
	LastOnDay  bool
	Section    Section
	Location   string
}

func LayoutTimelineEvent(layout Layout, rect render.Rect, event *TimelineEvent, scene *render.Scene, source *image.Source) render.Rect {

	// log.Println("layout event", len(event.Section.photos), rect.X, rect.Y)

	textHeight := 30.
	textBounds := render.Rect{
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

	headerText := event.StartTime.Format(startTimeFormat) + " " + event.Location

	duration := event.EndTime.Sub(event.StartTime)
	if duration >= 1*time.Minute {
		dur := durafmt.Parse(duration)
		headerText += "   " + dur.LimitFirstN(1).String()
	}

	font := scene.Fonts.Main.Face(40, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	text := render.NewTextFromRect(
		textBounds,
		&font,
		headerText,
	)
	text.VAlign = canvas.Bottom
	scene.Texts = append(scene.Texts, text)
	rect.Y += textHeight + 4

	newBounds := addSectionToScene(&event.Section, scene, rect, layout, source)

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += 10
	return rect
}

func LayoutTimeline(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.

	scene.Bounds.W = layout.ViewportWidth

	event := TimelineEvent{}
	eventCount := 0
	var lastPhotoTime time.Time
	var lastLocationTime time.Time
	var lastLatLng s2.LatLng

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

	locations := make(map[string]struct{})

	index := 0
	for info := range infos {
		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 30*time.Minute {
			event.StartTime = lastPhotoTime
			for location := range locations {
				if event.Location != "" {
					event.Location = event.Location + ", " + location
				} else {
					event.Location = location
				}
			}

			locations = make(map[string]struct{})

			rect = LayoutTimelineEvent(layout, rect, &event, scene, source)
			eventCount++
			event = TimelineEvent{
				EndTime: photoTime,
			}
		}
		lastPhotoTime = photoTime

		event.Section.infos = append(event.Section.infos, info)

		if source.Geo.Available() {
			lastLocationTimeElapsed := lastLocationTime.Sub(photoTime)
			if lastLocationTimeElapsed < 0 {
				lastLocationTimeElapsed = -lastLocationTimeElapsed
			}
			queryLocation := lastLocationTime.IsZero() || lastLocationTimeElapsed > 15*time.Minute
			if queryLocation && image.IsValidLatLng(info.LatLng) {
				lastLocationTime = photoTime
				dist := image.AngleToKm(lastLatLng.Distance(info.LatLng))
				if dist > 1 {
					location, err := source.Geo.ReverseGeocode(context.TODO(), info.LatLng)
					if err == nil {
						locations[location] = struct{}{}
					}
					lastLatLng = info.LatLng
				}
			}
		}

		layoutCounter.Set(index)
		index++
		scene.FileCount = index
	}
	layoutPlaced()

	if len(event.Section.infos) > 0 {
		event.StartTime = lastPhotoTime
		rect = LayoutTimelineEvent(layout, rect, &event, scene, source)
		event.Location = ""
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}

}
