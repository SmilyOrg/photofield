package photofield

import (
	"image"
	"image/color"
	"log"
	"math"
	"sort"
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

func layoutSection(section *Section, bounds canvas.Rect, imageHeight float64, source *storage.ImageSource) canvas.Point {
	imageSpacing := 0.
	lineSpacing := 0.
	x := 0.
	y := 0.
	for i := range section.photos {
		photo := section.photos[i]
		size := photo.Original.GetSize(source)

		aspectRatio := float64(size.X) / float64(size.Y)
		imageWidth := float64(imageHeight) * aspectRatio

		photo.Original.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			imageHeight,
			float64(size.X),
			float64(size.Y),
		)

		// fmt.Printf("%d %f %f %f\n", i, x, imageWidth, bounds.W)

		x += imageWidth + imageSpacing
		if x+imageWidth+imageSpacing > bounds.W {
			x = 0
			y += imageHeight + lineSpacing
		}
	}
	x = 0
	y += imageHeight + lineSpacing
	return canvas.Point{
		X: bounds.X + x,
		Y: bounds.Y + y,
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
	// imageSpacing := 6.
	// lineSpacing := 6.
	sceneMargin := 10.

	scene.Size.Width = 2000

	// cols := int(scene.size.width/(imageWidth+margin)) - 2

	log.Println("layout")
	lastLogTime := time.Now()

	x := sceneMargin
	y := sceneMargin
	for i := range days {
		day := days[i]
		p := layoutSection(day, canvas.Rect{X: x, Y: y, W: scene.Size.Width, H: scene.Size.Height}, imageHeight, source)
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
	Number int
	Bounds canvas.Rect
	Section
}

type Day struct {
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
		size := image.Point{X: info.Config.Width, Y: info.Config.Height}

		// weekday := info.DateTime.Weekday()
		// _, week := info.DateTime.ISOWeek()

		// weekBounds := getWeekBounds(info, imageHeight, canvas.Rect{
		// 	X: sceneMargin,
		// 	Y: sceneMargin,
		// 	W: scene.Size.Width - sceneMargin*2,
		// 	H: 0,
		// })

		_, weekNum := info.DateTime.ISOWeek()

		// week, weekSeen := weeks[weekNum]
		// if !weekSeen {
		if week == nil || week.Number != weekNum {
			week = &Week{
				Number: weekNum,
				Bounds: getColumnBounds(weekNum, 1, 30, canvas.Rect{
					X: sceneMargin,
					Y: sceneMargin,
					W: scene.Size.Width - sceneMargin*2,
					H: imageHeight,
				}),
			}
			// week = Week{Days: make(map[int]Day)}
			// weeks[weekNum] = week
			scene.Solids = append(scene.Solids, NewSolidFromRect(week.Bounds, color.Gray{Y: 0xFA}))
			// lastWeekNum = weekNum
			// lastWeekdayNum = -1
			// scene.Texts = append(scene.Texts, NewHeaderFromRect(week.Bounds, &headerFont,
			// 	fmt.Sprintf("CW %d", weekNum),
			// ))
		}

		dayNum := int(dateTime.Day())
		// weekdayNum := int(dateTime.Weekday()+6) % 7
		// day, daySeen := week.Days[weekdayNum]
		// if !daySeen {
		// 	day = Day{Hours: make(map[int]*Hour)}
		// 	week.Days[weekdayNum] = day
		// if lastWeekdayNum != weekdayNum {
		if day == nil || day.Number != dayNum {
			weekdayNum := int(dateTime.Weekday()+6) % 7

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

		hourNum := dateTime.Hour()
		// hour, hourSeen := day.Hours[hourNum]
		// if !hourSeen {
		// 	hour = &Hour{}
		// 	hour.Bounds = hourBounds
		// day.Hours[hourNum] = hour
		// if lastHourNum != hourNum {
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
			// hour = &Hour{}
			// hour.Bounds = hourBounds
		}
		hour.photos = append(hour.photos, photo)

		minuteBounds := getRowBounds(dateTime.Minute(), 60, 0, hour.Bounds)
		// secondBounds := getColumnBounds(dateTime.Second()/4, 60/4, 10, minuteBounds)

		// x = sceneMargin + float64(weekday)/weekdayCount*(scene.Size.Width-sceneMargin*2)
		// y = sceneMargin + float64(week)*(imageHeight+lineSpacing)

		// fmt.Printf("%4d %4d\n", size.X, size.Y)

		// aspectRatio := float64(size.X) / float64(size.Y)
		// imageWidth := float64(imageHeight) * aspectRatio
		// imageWidth := float64(imageHeight) * aspectRatio

		photo.Original.Sprite.PlaceFit(
			minuteBounds.X,
			minuteBounds.Y,
			minuteBounds.W,
			minuteBounds.H,
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
