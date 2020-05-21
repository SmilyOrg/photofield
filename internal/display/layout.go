package photofield

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"sort"
	"sync"
	"time"

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

	scene.Size = Size{
		Width:  float64(cols+2) * (imageWidth + margin),
		Height: math.Ceil(float64(rows+2)) * (imageHeight + margin),
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

func layoutSectionChannel(photos chan SectionPhoto, bounds canvas.Rect, boundsOut chan canvas.Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, scene *Scene, source *storage.ImageSource) {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	i := 0
	for photo := range photos {

		// log.Println("layout", photo.Index)

		aspectRatio := photo.Size.Width / photo.Size.Height
		imageWidth := float64(imageHeight) * aspectRatio

		if x+imageWidth+imageSpacing > bounds.W {
			x = 0
			y += imageHeight + lineSpacing
		}

		photo.Photo.Original.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			imageHeight,
			photo.Size.Width,
			photo.Size.Height,
		)

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
	boundsOut <- canvas.Rect{
		X: bounds.X,
		Y: bounds.Y,
		W: bounds.W,
		H: y,
	}
	close(boundsOut)
}

func layoutSection(section *Section, bounds canvas.Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, source *storage.ImageSource) canvas.Point {
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

func getSectionPhotos(id int, scene *Scene, index chan int, output chan SectionPhoto, wg *sync.WaitGroup, source *storage.ImageSource) {
	for i := range index {
		// log.Println("get", id, i)
		// if i == 0 {
		// 	time.Sleep(1 * time.Millisecond)
		// }
		photo := &scene.Photos[i]
		size := photo.Original.GetSize(source)
		output <- SectionPhoto{
			Index: i,
			Photo: photo,
			Size:  Size{Width: float64(size.X), Height: float64(size.Y)},
		}
	}
	wg.Done()
	// close(output)
}

func LayoutWallRegionSource(rect canvas.Rect, scene *Scene) []Region {
	fmt.Println(rect.String(), scene.Size.Width)
	regions := make([]Region, 0)
	photos := make(chan *Photo)
	go scene.GetVisiblePhotos(photos, rect)
	for photo := range photos {
		fmt.Println(photo.Original.Path)
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

	scene.Size = Size{
		Width:  float64(cols+2) * (imageWidth + margin),
		Height: math.Ceil(float64(rows+2)) * (imageHeight + margin),
	}

	fmt.Printf("%f %f\n", scene.Size.Width, scene.Size.Height)

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

	index := make(chan int, 1)
	unordered := make(chan SectionPhoto, 1)
	ordered := make(chan SectionPhoto, 1)
	boundsOut := make(chan canvas.Rect)

	concurrent := 100
	wg := &sync.WaitGroup{}
	wg.Add(concurrent)

	for i := 0; i < concurrent; i++ {
		go getSectionPhotos(i, scene, index, unordered, wg, source)
	}

	go orderSectionPhotoStream(unordered, ordered)
	go layoutSectionChannel(ordered, canvas.Rect{
		X: x,
		Y: y,
		W: scene.Size.Width - sceneMargin*2,
		H: scene.Size.Height - sceneMargin*2,
	}, boundsOut, imageHeight, imageSpacing, lineSpacing, scene, source)

	for i := range scene.Photos {
		index <- i
	}
	close(index)
	wg.Wait()
	close(unordered)
	newBounds := <-boundsOut

	scene.Size.Height = newBounds.Y + newBounds.H + sceneMargin
	scene.RegionSource = LayoutWallRegionSource

	// scene.RegionSource = func(rect canvas.Rect, scene *Scene) {
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

	// p := layoutSection(&section, canvas.Rect{
	// 	X: x,
	// 	Y: y,
	// 	W: scene.Size.Width - sceneMargin*2,
	// 	H: scene.Size.Height - sceneMargin*2,
	// }, imageHeight, imageSpacing, lineSpacing, source)

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
	imageSpacing := 6.
	lineSpacing := 6.
	sceneMargin := 10.

	scene.Size.Width = 2000

	// cols := int(scene.size.width/(imageWidth+margin)) - 2

	log.Println("layout")
	lastLogTime := time.Now()

	x := sceneMargin
	y := sceneMargin
	for i := range days {
		day := days[i]
		p := layoutSection(day, canvas.Rect{X: x, Y: y, W: scene.Size.Width, H: scene.Size.Height}, imageHeight, imageSpacing, lineSpacing, source)
		x = p.X
		y = p.Y
		// for i := range day.photos {
		// 	photo := day.photos[i]
		// 	size := photo.Original.GetSize(source)

		// 	// fmt.Printf("%4d %4d\n", size.X, size.Y)

		// 	aspectRatio := float64(size.X) / float64(size.Y)
		// 	imageWidth := float64(imageHeight) * aspectRatio

		// 	// photo.Place(x, y, imageWidth, imageHeight, source)
		// 	photo.Original.Sprite.PlaceFitHeight(x, y, imageHeight, float64(size.X), float64(size.Y))

		// 	x += imageWidth + imageSpacing
		// 	if x+imageWidth+imageSpacing > scene.Size.Width {
		// 		x = sceneMargin
		// 		y += imageHeight + lineSpacing
		// 	}

		now := time.Now()
		if now.Sub(lastLogTime) > 10000*time.Second {
			lastLogTime = now
			log.Printf("layout %d / %d\n", i, photoCount)
		}
		// }
		// x = sceneMargin
		// y += imageHeight + lineSpacing + 60
	}

	scene.Size.Height = y + sceneMargin

}

func getColumnBounds(value int, total int, spacing float64, bounds canvas.Rect) canvas.Rect {
	return canvas.Rect{
		X: bounds.X,
		Y: bounds.Y + float64(value)/float64(total)*(bounds.H+spacing),
		W: bounds.W,
		H: 1.0 / float64(total) * (bounds.H - spacing),
	}
}

func getRowBounds(value int, total int, spacing float64, bounds canvas.Rect) canvas.Rect {
	return canvas.Rect{
		X: bounds.X + float64(value)/float64(total)*(bounds.W),
		Y: bounds.Y,
		W: 1.0 / float64(total) * (bounds.W - spacing),
		H: bounds.H,
	}
}

type Hour struct {
	Section
	Number int
	Bounds canvas.Rect
}

type Day struct {
	Section
	Number int
	Bounds canvas.Rect
	Hours  map[int]*Hour
}

type Week struct {
	Number int
	Bounds canvas.Rect
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

	scene.Size.Width = 2000

	// cols := int(scene.size.width/(imageWidth+margin)) - 2

	log.Println("layout")
	lastLogTime := time.Now()

	// weeks := make(map[int]Week)

	fontFamily := canvas.NewFontFamily("Roboto")
	// fontFamily.Use(canvas.CommonLigatures)
	err := fontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
	if err != nil {
		panic(err)
	}

	headerFont := fontFamily.Face(96.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)
	hourFont := fontFamily.Face(24.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)

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
		size := image.Point{X: info.Width, Y: info.Height}

		_, weekNum := info.DateTime.ISOWeek()

		// lastWeek := week

		if week == nil || week.Number != weekNum {
			week = &Week{
				Number: weekNum,
				Bounds: getColumnBounds(weekNum, 1, 30, canvas.Rect{
					X: sceneMargin,
					Y: sceneMargin,
					W: scene.Size.Width - sceneMargin*2,
					H: imageHeight,
				}),
				// Bounds: canvas.Rect{
				// 	X: sceneMargin,
				// 	Y: -1,
				// 	W: scene.Size.Width - sceneMargin*2,
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
			scene.Texts = append(scene.Texts, NewHeaderFromRect(day.Bounds, &headerFont,
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
			scene.Texts = append(scene.Texts, NewTextFromRect(hourBoundsOriginal.Move(canvas.Point{X: 2, Y: 0}), &hourFont,
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

	scene.Size.Height = y + sceneMargin

}
