package photofield

import (
	"log"
	"time"

	. "photofield/internal"
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

func LayoutTimelineEvent(config LayoutConfig, rect Rect, event *TimelineEvent, scene *Scene, source *storage.ImageSource) Rect {

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

	font := config.FontFamily.Face(40, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	scene.Texts = append(scene.Texts,
		NewTextFromRect(
			textBounds,
			&font,
			headerText,
		),
	)
	rect.Y += textHeight + 15

	photos := make(chan SectionPhoto, 1)
	boundsOut := make(chan Rect)
	// event.Section.Inverted = true
	go layoutSectionPhotos(photos, rect, boundsOut, config, scene, source)
	go getSectionPhotos(&event.Section, photos, source)
	newBounds := <-boundsOut

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += 10
	return rect
}

func LayoutTimeline(config LayoutConfig, scene *Scene, source *storage.ImageSource) {

	// log.Println("layout")

	// log.Println("layout load info")
	layoutPhotos := getLayoutPhotos(scene.Photos, source)
	sortNewestToOldest(layoutPhotos)

	count := len(layoutPhotos)
	if config.Limit > 0 && config.Limit < count {
		count = config.Limit
	}

	config.ImageSpacing = 0.02 * config.ImageHeight
	config.LineSpacing = 0.02 * config.ImageHeight

	scene.Photos = scene.Photos[0:count]
	layoutPhotos = layoutPhotos[0:count]

	sceneMargin := 10.

	scene.Bounds.W = config.SceneWidth

	event := TimelineEvent{}
	eventCount := 0
	var lastPhotoTime time.Time

	rect := Rect{
		X: sceneMargin,
		Y: sceneMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	if config.FontFamily == nil {
		config.FontFamily = canvas.NewFontFamily("Roboto")
		err := config.FontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
		if err != nil {
			panic(err)
		}
		err = config.FontFamily.LoadFontFile("fonts/Roboto/Roboto-Bold.ttf", canvas.FontBold)
		if err != nil {
			panic(err)
		}
	}
	if config.HeaderFont == nil {
		face := config.FontFamily.Face(80.0, canvas.Gray, canvas.FontRegular, canvas.FontNormal)
		config.HeaderFont = &face
	}

	scene.Solids = make([]Solid, 0)
	scene.Texts = make([]Text, 0)

	// log.Println("layout placing")
	layoutPlaced := ElapsedWithCount("layout placing", count)
	lastLogTime := time.Now()

	scene.Photos = scene.Photos[:0]
	for i := range layoutPhotos {
		if i >= count {
			break
		}
		LayoutPhoto := &layoutPhotos[i]
		info := LayoutPhoto.Info
		scene.Photos = append(scene.Photos, LayoutPhoto.Photo)
		photo := &scene.Photos[len(scene.Photos)-1]

		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 30*time.Minute {
			event.StartTime = lastPhotoTime
			rect = LayoutTimelineEvent(config, rect, &event, scene, source)
			eventCount++
			event = TimelineEvent{
				EndTime: photoTime,
			}
		}
		lastPhotoTime = photoTime

		event.Section.photos = append(event.Section.photos, photo)

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, count)
		}
	}
	layoutPlaced()

	if len(event.Section.photos) > 0 {
		event.StartTime = lastPhotoTime
		rect = LayoutTimelineEvent(config, rect, &event, scene, source)
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		imageSource: source,
	}

}
