package photofield

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"path/filepath"
	"sort"
	"sync"
	"time"

	. "photofield/internal"
	storage "photofield/internal/storage"

	"github.com/tdewolff/canvas"
)

func LayoutSquare(scene *Scene, source *storage.ImageSource) {

	// imageWidth := 120.
	photoCount := len(scene.Photos)

	imageWidth := 100.
	imageHeight := imageWidth * 2 / 3

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

type Section struct {
	photos []*Photo
}

type SectionPhoto struct {
	Index int
	Photo *Photo
	Size  Size
}

func layoutFitRow(row []SectionPhoto, bounds Rect, imageSpacing float64) float64 {
	count := len(row)
	if count == 0 {
		return 1.
	}
	firstPhoto := row[0]
	firstRect := firstPhoto.Photo.Original.Sprite.Rect
	lastPhoto := row[count-1]
	lastRect := lastPhoto.Photo.Original.Sprite.Rect
	totalSpacing := float64(count-1) * imageSpacing

	rowWidth := lastRect.X + lastRect.W
	scale := (bounds.W - totalSpacing) / (rowWidth - totalSpacing)
	x := firstRect.X
	for i := range row {
		photo := row[i]
		rect := photo.Photo.Original.Sprite.Rect
		photo.Photo.Original.Sprite.Rect = Rect{
			X: x,
			Y: rect.Y,
			W: rect.W * scale,
			H: rect.H * scale,
		}
		x += photo.Photo.Original.Sprite.Rect.W + imageSpacing
	}

	// fmt.Printf("fit row width %5.2f / %5.2f -> %5.2f  scale %.2f\n", rowWidth, bounds.W, lastPhoto.Photo.Original.Sprite.Rect.X+lastPhoto.Photo.Original.Sprite.Rect.W, scale)

	x -= imageSpacing
	return scale
}

func layoutSectionChannel(photos chan SectionPhoto, bounds Rect, boundsOut chan Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, scene *Scene, source *storage.ImageSource) {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	i := 0

	row := make([]SectionPhoto, 0)

	for photo := range photos {

		// log.Println("layout", photo.Index)

		aspectRatio := float64(photo.Size.X) / float64(photo.Size.Y)
		imageWidth := float64(imageHeight) * aspectRatio

		if x+imageWidth > bounds.W {
			scale := layoutFitRow(row, bounds, imageSpacing)
			row = nil
			x = 0
			y += imageHeight*scale + lineSpacing
		}

		// fmt.Printf("%4.0f %4.0f %4.0f %4.0f %4.0f %4.0f %4.0f\n", bounds.X, bounds.Y, x, y, imageHeight, photo.Size.Width, photo.Size.Height)

		photo.Photo.Original.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			imageHeight,
			float64(photo.Size.X),
			float64(photo.Size.Y),
		)

		row = append(row, photo)

		// photoRect := photo.Photo.Original.Sprite.GetBounds()
		// scene.Regions = append(scene.Regions, Region{
		// 	Id: len(scene.Regions),
		// 	Bounds: Bounds{
		// 		X: photoRect.X,
		// 		Y: photoRect.Y,
		// 		W: photoRect.W,
		// 		H: photoRect.H,
		// 	},
		// })

		// fmt.Printf("%d %f %f %f\n", i, x, imageWidth, bounds.W)

		x += imageWidth + imageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout section %d\n", photo.Index)
		}
		i++
	}
	x = 0
	y += imageHeight + lineSpacing
	boundsOut <- Rect{
		X: bounds.X,
		Y: bounds.Y,
		W: bounds.W,
		H: y,
	}
	close(boundsOut)
}

func layoutSection(section *Section, bounds Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, source *storage.ImageSource) canvas.Point {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	photoCount := len(section.photos)
	for i := range section.photos {
		photo := section.photos[i]
		size := photo.Original.GetSize(source)

		aspectRatio := float64(size.X) / float64(size.Y)
		imageWidth := float64(imageHeight) * aspectRatio

		if x+imageWidth+imageSpacing > bounds.W {
			x = 0
			y += imageHeight + lineSpacing
		}

		photo.Original.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			imageHeight,
			float64(size.X),
			float64(size.Y),
		)

		// fmt.Printf("%d %f %f %f\n", i, x, imageWidth, bounds.W)

		x += imageWidth + imageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout section %d / %d\n", i, photoCount)
		}
	}
	x = 0
	y += imageHeight + lineSpacing
	return canvas.Point{
		X: bounds.X + x,
		Y: bounds.Y + y,
	}
}

func orderSectionPhotoStream(input chan SectionPhoto, output chan SectionPhoto) {
	var buffer []SectionPhoto
	index := 0
	for photo := range input {

		if photo.Index != index {
			buffer = append(buffer, photo)
			// log.Println("buffer", len(buffer))
			continue
		}

		// log.Println("order", index, photo.Index)
		output <- photo
		index++

		found := true
		for found == true {
			found = false
			// log.Println("order buffer before", len(buffer))
			for i := range buffer {
				bphoto := buffer[i]
				// log.Println("order search", index, bphoto.Index)
				if bphoto.Index == index {
					// log.Println("order search", index, "found")
					// log.Println("order", index, bphoto.Index)
					output <- bphoto
					index++
					lastIndex := len(buffer) - 1
					// log.Println("order replace", buffer[i].Index, "at", i, "with", buffer[lastIndex].Index, "at", lastIndex)
					buffer[i] = buffer[lastIndex]
					// buffer[lastIndex] = nil
					buffer = buffer[:lastIndex]
					found = true
					break
				}
			}
			// log.Println("order buffer after", len(buffer))
			if !found {
				// log.Println("order search", index, "not found")
			}
		}
		// log.Println("buffer", len(buffer))

	}
	close(output)
}

func getSectionPhotosUnordered(id int, section *Section, index chan int, output chan SectionPhoto, wg *sync.WaitGroup, source *storage.ImageSource) {
	for i := range index {
		photo := section.photos[i]
		size := photo.Original.GetSize(source)
		output <- SectionPhoto{
			Index: i,
			Photo: photo,
			Size:  size,
		}
	}
	wg.Done()
}

func getSectionPhotos(section *Section, output chan SectionPhoto, source *storage.ImageSource) {
	index := make(chan int, 1)
	unordered := make(chan SectionPhoto, 1)

	concurrent := 100
	wg := &sync.WaitGroup{}
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go getSectionPhotosUnordered(i, section, index, unordered, wg, source)
	}
	go orderSectionPhotoStream(unordered, output)

	for i := range section.photos {
		index <- i
	}
	close(index)
	wg.Wait()
	close(unordered)
}

// func getPhotosSize(id int, scene *Scene, index chan int, output chan SectionPhoto, wg *sync.WaitGroup, source *storage.ImageSource) {
// 	for i := range index {
// 		photo := &scene.Photos[i]
// 		photo.Original.Size = photo.Original.GetSize(source)
// 	}
// 	wg.Done()
// }

type PhotoRegionData struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

type LayoutWallRegionSource struct {
}

func (regionSource *LayoutWallRegionSource) GetRegionsFromBounds(rect Rect, scene *Scene, regionConfig RegionConfig) []Region {
	// fmt.Println(rect.String(), scene.Size.Width)
	regions := make([]Region, 0)
	photos := make(chan PhotoRef)
	go scene.GetVisiblePhotos(photos, rect, regionConfig.MaxCount)
	for photo := range photos {
		regions = append(regions, Region{
			Id:     photo.Index,
			Bounds: photo.Photo.Original.Sprite.Rect,
			Data: PhotoRegionData{
				Path:     photo.Photo.Original.Path,
				Filename: filepath.Base(photo.Photo.Original.Path),
			},
		})
	}
	return regions
	// return Region{
	// 	Id: 1,
	// 	Bounds: Bounds{
	// 		X: 1,
	// 		Y: 2,
	// 		W: 3,
	// 		H: 4,
	// 	},
	// }
}

func LayoutWall(config *Config, scene *Scene, source *storage.ImageSource) {

	// photoCount := len(scene.Photos)

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

	// imageWidth := 200.
	// imageHeight := 160.

	// edgeCount := int(math.Sqrt(float64(photoCount)))

	// margin := 1.

	// cols := edgeCount
	// rows := int(math.Ceil(float64(photoCount) / float64(cols)))

	// scene.Size = Size{
	// 	Width:  10000,
	// 	Height: 0,
	// }

	// cols := int(scene.size.width/(imageWidth+margin)) - 2

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
	go layoutSectionChannel(photos, Rect{
		X: x,
		Y: y,
		W: scene.Bounds.W - sceneMargin*2,
		H: scene.Bounds.H - sceneMargin*2,
	}, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)
	getSectionPhotos(&section, photos, source)

	newBounds := <-boundsOut

	scene.Bounds.H = newBounds.Y + newBounds.H + sceneMargin
	// scene.RegionSource = LayoutWallRegionSource

	// scene.RegionSource = func(rect Rect, scene *Scene) {
	// }

	// scene.NormalizeRegions()

	// close(index)

	// photos := make(chan SectionPhoto, 1)

	// concurrent := 1
	// for i := 0; i < concurrent; i++ {
	// 	go drawPhotoChannel(i, index, config, scene, c, scales, source)
	// }
	// for i := range scene.Photos {
	// 	index <- i
	// }

	// p := layoutSection(&section, Rect{
	// 	X: x,
	// 	Y: y,
	// 	W: scene.Size.Width - sceneMargin*2,
	// 	H: scene.Size.Height - sceneMargin*2,
	// }, imageHeight, imageSpacing, lineSpacing, source)

}

type PhotoRegionSource struct {
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect Rect, scene *Scene, regionConfig RegionConfig) []Region {
	regions := make([]Region, 0)
	photos := make(chan PhotoRef)
	go scene.GetVisiblePhotos(photos, rect, regionConfig.MaxCount)
	for photo := range photos {
		regions = append(regions, Region{
			Id:     photo.Index,
			Bounds: photo.Photo.Original.Sprite.Rect,
			Data: PhotoRegionData{
				Path:     photo.Photo.Original.Path,
				Filename: filepath.Base(photo.Photo.Original.Path),
			},
		})
	}
	return regions
}

func (regionSource PhotoRegionSource) GetRegionById(id int, scene *Scene, regionConfig RegionConfig) Region {
	if id < 0 || id >= len(scene.Photos)-1 {
		return Region{Id: -1}
	}
	photo := scene.Photos[id]
	return Region{
		Id:     id,
		Bounds: photo.Original.Sprite.Rect,
		Data: PhotoRegionData{
			Path:     photo.Original.Path,
			Filename: filepath.Base(photo.Original.Path),
		},
	}
}

func LayoutTimeline(config *Config, scene *Scene, source *storage.ImageSource) {

	// imageWidth := 120.
	photoCount := len(scene.Photos)

	sort.Slice(scene.Photos, func(i, j int) bool {
		a := source.GetImageInfo(scene.Photos[i].Original.Path)
		b := source.GetImageInfo(scene.Photos[j].Original.Path)
		return a.DateTime.Before(b.DateTime)
	})

	sectionMap := make(map[string]*Section)
	var days []*Section

	for i := range scene.Photos {
		photo := &scene.Photos[i]
		info := source.GetImageInfo(photo.Original.Path)
		dayId := info.DateTime.Format("2006-02-01")
		day := sectionMap[dayId]
		if day == nil {
			day = &Section{}
			days = append(days, day)
			sectionMap[dayId] = day
		}
		day.photos = append(day.photos, photo)
	}

	// imageWidth := 200.
	imageHeight := 160.
	imageSpacing := 3.
	lineSpacing := 3.
	sceneMargin := 10.

	scene.Bounds.W = 2000

	log.Println("layout")
	lastLogTime := time.Now()

	x := sceneMargin
	y := sceneMargin
	for i := range days {
		day := days[i]

		photos := make(chan SectionPhoto, 1)
		boundsOut := make(chan Rect)
		go layoutSectionChannel(photos, Rect{
			X: x,
			Y: y,
			W: scene.Bounds.W - sceneMargin*2,
			H: scene.Bounds.H - sceneMargin*2,
		}, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)
		go getSectionPhotos(day, photos, source)

		newBounds := <-boundsOut

		scene.Bounds.H = newBounds.Y + newBounds.H + sceneMargin
		// scene.RegionSource = LayoutWallRegionSource

		x = newBounds.X
		y = newBounds.Y + newBounds.H

		now := time.Now()
		if now.Sub(lastLogTime) > 10000*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, photoCount)
		}
	}

	scene.Bounds.H = y + sceneMargin
	// scene.RegionSource = LayoutTimelineRegionSource

}

type Event struct {
	StartTime time.Time
	Section   Section
}

func LayoutTimelineEvent(rect Rect, event *Event, headerFont *canvas.FontFace, fontFamily *canvas.FontFamily, scene *Scene, source *storage.ImageSource) Rect {

	imageHeight := 160.
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
	font := fontFamily.Face(70.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)

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
	go layoutSectionChannel(photos, rect, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)
	go getSectionPhotos(&event.Section, photos, source)
	newBounds := <-boundsOut

	rect.Y = newBounds.Y + newBounds.H
	rect.Y += 160
	return rect
}

type TimelinePhoto struct {
	Index int
	Photo Photo
	Info  ImageInfo
}

func getTimelinePhotosUnordered(id int, photoRefs chan PhotoRef, output chan TimelinePhoto, wg *sync.WaitGroup, source *storage.ImageSource) {
	// lastLogTime := time.Now()
	// lastIndex := -1
	// interval := 5 * time.Second
	for photoRef := range photoRefs {
		// if lastIndex == -1 {
		// 	lastIndex = photoRef.Index
		// }
		info := source.GetImageInfo(photoRef.Photo.Original.Path)
		// now := time.Now()
		// if now.Sub(lastLogTime) > interval {
		// 	perSec := float64(photoRef.Index-lastIndex) / interval.Seconds()
		// 	log.Printf("layout load info %d, %.2f / sec (goroutine %d)\n", photoRef.Index, perSec, id)
		// 	lastLogTime = now
		// 	lastIndex = photoRef.Index
		// }
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
	photoRefs := make(chan PhotoRef, 1)
	unordered := make(chan TimelinePhoto, 1)

	concurrent := 10
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

func LayoutTimelineEvents(config *Config, scene *Scene, source *storage.ImageSource) {

	log.Println("layout")

	// imageWidth := 120.
	photoCount := len(scene.Photos)

	log.Println("layout load info")
	timelinePhotos := getTimelinePhotos(scene.Photos, source)

	log.Println("layout sort")
	sort.Slice(timelinePhotos, func(i, j int) bool {
		a := timelinePhotos[i]
		b := timelinePhotos[j]
		return a.Info.DateTime.After(b.Info.DateTime)
	})

	// for i := range timelinePhotos {
	// 	scene.Photos[i] = timelinePhotos[i].Photo
	// 	// println(timelinePhotos[i].Info.DateTime.Format("2006-02-01"), timelinePhotos[i].Photo.Original.Path)
	// }

	// sort.Slice(scene.Photos, func(i, j int) bool {
	// 	a := source.GetImageInfo(scene.Photos[i].Original.Path)
	// 	b := source.GetImageInfo(scene.Photos[j].Original.Path)
	// 	return a.DateTime.After(b.DateTime)
	// })

	sceneMargin := 10.

	scene.Bounds.W = 2000
	// scene.Bounds.W = 10000

	event := Event{}
	var lastPhotoTime time.Time

	rect := Rect{
		X: sceneMargin,
		Y: sceneMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	fontFamily := canvas.NewFontFamily("Roboto")
	err := fontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
	if err != nil {
		panic(err)
	}
	headerFont := fontFamily.Face(80.0, canvas.Gray, canvas.FontRegular, canvas.FontNormal)

	log.Println("layout placing")
	lastLogTime := time.Now()
	for i := range scene.Photos {
		timelinePhoto := timelinePhotos[i]
		scene.Photos[i] = timelinePhoto.Photo
		photo := &scene.Photos[i]
		info := timelinePhoto.Info
		photoTime := info.DateTime
		elapsed := lastPhotoTime.Sub(photoTime)
		if elapsed > 10*time.Minute {

			rect = LayoutTimelineEvent(rect, &event, &headerFont, fontFamily, scene, source)

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
		rect = LayoutTimelineEvent(rect, &event, &headerFont, fontFamily, scene, source)
	}

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{}

}

func getColumnBounds(value int, total int, spacing float64, bounds Rect) Rect {
	return Rect{
		X: bounds.X,
		Y: bounds.Y + float64(value)/float64(total)*(bounds.H+spacing),
		W: bounds.W,
		H: 1.0 / float64(total) * (bounds.H - spacing),
	}
}

func getRowBounds(value int, total int, spacing float64, bounds Rect) Rect {
	return Rect{
		X: bounds.X + float64(value)/float64(total)*(bounds.W),
		Y: bounds.Y,
		W: 1.0 / float64(total) * (bounds.W - spacing),
		H: bounds.H,
	}
}

type Hour struct {
	Section
	Number int
	Bounds Rect
}

type Day struct {
	Section
	Number int
	Bounds Rect
	Hours  map[int]*Hour
}

type Week struct {
	Number int
	Bounds Rect
	Days   map[int]Day
}

func LayoutCalendar(config *Config, scene *Scene, source *storage.ImageSource) {

	// imageWidth := 120.
	photoCount := len(scene.Photos)

	sort.Slice(scene.Photos, func(i, j int) bool {
		a := source.GetImageInfo(scene.Photos[i].Original.Path)
		b := source.GetImageInfo(scene.Photos[j].Original.Path)
		return a.DateTime.Before(b.DateTime)
	})

	// imageWidth := 200.
	// imageHeight := 160.
	imageHeight := 300.
	// imageHeight := 50.
	// imageSpacing := 6.
	// lineSpacing := 6.
	sceneMargin := 10.

	scene.Bounds.W = 2000

	// cols := int(scene.size.width/(imageWidth+margin)) - 2

	log.Println("layout")
	lastLogTime := time.Now()

	// weeks := make(map[int]Week)

	// var lastHour *Hour

	var week *Week
	var day *Day
	var hour *Hour

	// x := sceneMargin
	y := sceneMargin
	for i := range scene.Photos {
		photo := &scene.Photos[i]
		info := source.GetImageInfo(photo.Original.Path)
		dateTime := info.DateTime
		size := Size{X: info.Width, Y: info.Height}

		_, weekNum := info.DateTime.ISOWeek()

		// lastWeek := week

		if week == nil || week.Number != weekNum {
			week = &Week{
				Number: weekNum,
				Bounds: getColumnBounds(weekNum, 1, 30, Rect{
					X: sceneMargin,
					Y: sceneMargin,
					W: scene.Bounds.W - sceneMargin*2,
					H: imageHeight,
				}),
				// Bounds: Rect{
				// 	X: sceneMargin,
				// 	Y: -1,
				// 	W: scene.Bounds.Width - sceneMargin*2,
				// 	H: imageHeight,
				// },
			}
			// y += imageHeight
			// scene.Solids = append(scene.Solids, NewSolidFromRect(week.Bounds, color.Gray{Y: 0xFA}))
			// scene.Texts = append(scene.Texts, NewHeaderFromRect(week.Bounds, &headerFont,
			// 	fmt.Sprintf("CW %d", weekNum),
			// ))
		}

		dayNum := int(dateTime.Day())
		if day == nil || day.Number != dayNum {
			weekdayNum := int(dateTime.Weekday()+6) % 7

			// if day != nil {
			// 	sectionEnd := layoutSection(&day.Section, day.Bounds, 20, source)
			// 	if sectionEnd.Y > lastWeek.Bounds.Y+lastWeek.Bounds.H {
			// 		lastWeek.Bounds.H = sectionEnd.Y - lastWeek.Bounds.Y
			// 	}
			// }
			// if week.Bounds.Y == -1 {
			// 	if lastWeek != nil {
			// 		week.Bounds.Y = lastWeek.Bounds.Y + lastWeek.Bounds.H
			// 	} else {
			// 		week.Bounds.Y = sceneMargin
			// 	}
			// }

			day = &Day{
				Number: dayNum,
				Bounds: getRowBounds(weekdayNum, 7, 20, week.Bounds),
			}

			scene.Solids = append(scene.Solids, NewSolidFromRect(day.Bounds, color.Gray{Y: 0xF0}))
			dayNum := dateTime.Day()
			dayFormat := "2"
			if dayNum == 1 {
				dayFormat = "2 Jan"
			}
			scene.Texts = append(scene.Texts, NewTextFromRect(day.Bounds, &scene.Fonts.Header,
				dateTime.Format(dayFormat),
			))
		}
		day.photos = append(day.photos, photo)

		hourNum := dateTime.Hour()
		if hour == nil || hour.Number != hourNum {

			hour = &Hour{
				Number: hourNum,
				Bounds: getColumnBounds(hourNum, 24, 10, day.Bounds),
			}

			hourBoundsIndent := 24.
			hourBoundsOriginal := hour.Bounds
			hour.Bounds.X += hourBoundsIndent
			hour.Bounds.W -= hourBoundsIndent

			scene.Solids = append(scene.Solids, NewSolidFromRect(hour.Bounds, color.Gray{Y: 0xE0}))
			scene.Texts = append(scene.Texts, NewTextFromRect(hourBoundsOriginal.Move(Point{X: 2, Y: 0}), &scene.Fonts.Hour,
				dateTime.Format("15:00"),
			))

			// if hour != nil {
			// 	layoutSection(&hour.Section, hour.Bounds, hour.Bounds.H/3, source)
			// }
		}
		// hour.photos = append(hour.photos, photo)

		minuteBounds := getRowBounds(dateTime.Minute(), 60, 0, hour.Bounds)
		// secondBounds := getColumnBounds(dateTime.Second()/10, 60/10, 0, minuteBounds)

		// secondBounds := getRowBounds(dateTime.Minute()*60+dateTime.Second(), 60*60, 0, hour.Bounds)

		photoBounds := minuteBounds

		photo.Original.Sprite.PlaceFit(
			photoBounds.X,
			photoBounds.Y,
			photoBounds.W,
			photoBounds.H,
			float64(size.X),
			float64(size.Y),
		)

		// photo.Original.Sprite.PlaceFitHeight(
		// 	weekdayBounds.X,
		// 	weekdayBounds.Y,
		// 	weekdayBounds.H,
		// 	float64(size.X),
		// 	float64(size.Y),
		// )

		if day.Bounds.Y+week.Bounds.H > y {
			y = day.Bounds.Y + week.Bounds.H
		}

		// photo.Place(x, y, imageWidth, imageHeight, source)
		// photo.Original.Sprite.PlaceFitHeight(x, y, imageHeight, float64(size.X), float64(size.Y))

		// x += imageWidth + imageSpacing
		// if x+imageWidth+imageSpacing > scene.Size.Width {
		// 	x = sceneMargin
		// 	y += imageHeight + lineSpacing
		// }

		now := time.Now()
		if now.Sub(lastLogTime) > 10000*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, photoCount)
		}
	}

	// x = sceneMargin
	// y += imageHeight + lineSpacing

	scene.Bounds.H = y + sceneMargin

}
