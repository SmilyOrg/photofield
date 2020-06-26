package photofield

import (
	"log"
	"sort"
	"sync"
	"time"

	. "photofield/internal"
	. "photofield/internal/display"
	storage "photofield/internal/storage"

	"github.com/tdewolff/canvas"
)

type Event struct {
	StartTime time.Time
	Section   Section
}

type TimelinePhoto struct {
	Index int
	Photo Photo
	Info  ImageInfo
}

func getTimelinePhotosUnordered(id int, photoRefs chan PhotoRef, output chan TimelinePhoto, wg *sync.WaitGroup, source *storage.ImageSource) {
	for photoRef := range photoRefs {
		info := source.GetImageInfo(photoRef.Photo.Original.Path)
		output <- TimelinePhoto{
			Index: photoRef.Index,
			Photo: *photoRef.Photo,
			Info:  *info,
		}
	}
	wg.Done()
}

func getPhotoRefs(photos []Photo, output chan PhotoRef, wg *sync.WaitGroup) {
	for i := range photos {
		photo := &photos[i]
		output <- PhotoRef{
			Index: i,
			Photo: photo,
		}
	}
	close(output)
	wg.Done()
}

func getTimelinePhotosSlice(photos chan TimelinePhoto, output chan []TimelinePhoto) {
	var timelinePhotos []TimelinePhoto
	lastIndex := -1
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	for photo := range photos {
		now := time.Now()
		if now.Sub(lastLogTime) > logInterval {
			perSec := float64(photo.Index-lastIndex) / logInterval.Seconds()
			log.Printf("layout load info %d, %.2f / sec\n", photo.Index, perSec)
			lastLogTime = now
			lastIndex = photo.Index
		}
		timelinePhotos = append(timelinePhotos, photo)
	}
	output <- timelinePhotos
}

func getTimelinePhotos(photos []Photo, source *storage.ImageSource) []TimelinePhoto {
	defer ElapsedWithCount("layout load info", len(photos))()

	photoRefs := make(chan PhotoRef, 10)
	unordered := make(chan TimelinePhoto, 10)

	concurrent := 20
	wg := &sync.WaitGroup{}
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go getTimelinePhotosUnordered(i, photoRefs, unordered, wg, source)
	}
	wg.Add(1)
	go getPhotoRefs(photos, photoRefs, wg)

	timelinePhotosChan := make(chan []TimelinePhoto)
	go getTimelinePhotosSlice(unordered, timelinePhotosChan)

	wg.Wait()
	close(unordered)

	// sort.Slice(scene.Photos, func(i, j int) bool {
	// 	a := source.GetImageInfo(scene.Photos[i].Original.Path)
	// 	b := source.GetImageInfo(scene.Photos[j].Original.Path)
	// 	return a.DateTime.After(b.DateTime)
	// })

	return <-timelinePhotosChan
}

func LayoutTimelineEvent(config LayoutConfig, rect Rect, event *Event, scene *Scene, source *storage.ImageSource) Rect {

	imageHeight := config.ImageHeight
	imageSpacing := 3.
	lineSpacing := 3.

	// log.Println("layout event", len(event.Section.photos), rect.X, rect.Y)

	textHeight := 30.
	textBounds := Rect{
		X: rect.X,
		Y: rect.Y,
		W: rect.W,
		H: textHeight,
	}

	timeFormat := "Mon, Jan 2, 15:04"
	if event.StartTime.Year() != time.Now().Year() {
		timeFormat = "Mon, Jan 2, 2006, 15:04"
	}
	font := config.FontFamily.Face(70.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	scene.Texts = append(scene.Texts,
		NewTextFromRect(
			textBounds,
			&font,
			event.StartTime.Format(timeFormat),
		),
	)
	rect.Y += textHeight + 30

	photos := make(chan SectionPhoto, 1)
	boundsOut := make(chan Rect)
	go layoutSectionPhotos(photos, rect, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)
	go getSectionPhotos(&event.Section, photos, source)
	newBounds := <-boundsOut

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += imageHeight
	return rect
}

func LayoutTimelineEvents(config LayoutConfig, scene *Scene, source *storage.ImageSource) {

	log.Println("layout")

	photoCount := len(scene.Photos)

	log.Println("layout load info")
	timelinePhotos := getTimelinePhotos(scene.Photos, source)

	log.Println("layout sort")
	sort.Slice(timelinePhotos, func(i, j int) bool {
		a := timelinePhotos[i]
		b := timelinePhotos[j]
		return a.Info.DateTime.After(b.Info.DateTime)
	})

	sceneMargin := 10.

	scene.Bounds.W = config.SceneWidth

	event := Event{}
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
	}
	if config.HeaderFont == nil {
		face := config.FontFamily.Face(80.0, canvas.Gray, canvas.FontRegular, canvas.FontNormal)
		config.HeaderFont = &face
	}

	scene.Solids = make([]Solid, 0)
	scene.Texts = make([]Text, 0)

	log.Println("layout placing")
	lastLogTime := time.Now()
	for i := range scene.Photos {
		timelinePhoto := &timelinePhotos[i]
		scene.Photos[i] = timelinePhoto.Photo
		photo := &scene.Photos[i]
		info := timelinePhoto.Info

		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 10*time.Minute {
			rect = LayoutTimelineEvent(config, rect, &event, scene, source)
			event = Event{}
			event.StartTime = photoTime
		}
		lastPhotoTime = photoTime

		event.Section.photos = append(event.Section.photos, photo)

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, photoCount)
		}
	}

	if len(event.Section.photos) > 0 {
		rect = LayoutTimelineEvent(config, rect, &event, scene, source)
	}

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		imageSource: source,
	}

}
