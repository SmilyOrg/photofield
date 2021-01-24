package photofield

import (
	// . "photofield/internal"

	"log"

	. "photofield/internal"
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

func LayoutAlbumEvent(config LayoutConfig, rect Rect, event *AlbumEvent, scene *Scene, source *storage.ImageSource) Rect {

	// log.Println("layout event", len(event.Section.photos), rect.X, rect.Y)

	if event.FirstOnDay {
		font := config.FontFamily.Face(70, canvas.Black, canvas.FontRegular, canvas.FontNormal)
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

	font := config.FontFamily.Face(50, canvas.Black, canvas.FontRegular, canvas.FontNormal)
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

	photos := make(chan SectionPhoto, 1)
	boundsOut := make(chan Rect)
	go layoutSectionPhotos(photos, rect, boundsOut, config, scene, source)
	go getSectionPhotos(&event.Section, photos, source)
	newBounds := <-boundsOut

	rect.Y = newBounds.Y + newBounds.H
	if event.LastOnDay {
		rect.Y += 40
	} else {
		rect.Y += 6
	}
	return rect
}

func LayoutAlbum(config LayoutConfig, scene *Scene, source *storage.ImageSource) {

	layoutPhotos := getLayoutPhotos(scene.Photos, source)
	sortOldestToNewest(layoutPhotos)

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
		elapsed := photoTime.Sub(lastPhotoTime)
		if elapsed > 1*time.Hour {
			event.EndTime = lastPhotoTime
			event.LastOnDay = !SameDay(lastPhotoTime, photoTime)
			rect = LayoutAlbumEvent(config, rect, &event, scene, source)
			eventCount++
			event = AlbumEvent{
				First:      eventCount == 1,
				StartTime:  photoTime,
				FirstOnDay: !SameDay(lastPhotoTime, photoTime),
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
		event.EndTime = lastPhotoTime
		event.LastOnDay = true
		rect = LayoutAlbumEvent(config, rect, &event, scene, source)
		eventCount++
	}

	log.Printf("layout events %d\n", eventCount)

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		imageSource: source,
	}

}
