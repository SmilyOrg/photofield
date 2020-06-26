package photofield

import (
	"image/color"
	"log"
	"sort"
	"time"

	. "photofield/internal"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
)

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

func LayoutCalendar(config *RenderConfig, scene *Scene, source *storage.ImageSource) {

	// imageWidth := 120.
	photoCount := len(scene.Photos)

	sort.Slice(scene.Photos, func(i, j int) bool {
		a := source.GetImageInfo(source.GetImagePath(scene.Photos[i].Id))
		b := source.GetImageInfo(source.GetImagePath(scene.Photos[i].Id))
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
		info := source.GetImageInfo(source.GetImagePath(scene.Photos[i].Id))
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

		photo.Sprite.PlaceFit(
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
