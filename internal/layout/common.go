package photofield

import (
	"log"
	"path/filepath"
	. "photofield/internal"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tdewolff/canvas"
)

type LayoutConfig struct {
	Limit        int    `json:"limit"`
	Type         string `json:"type"`
	FontFamily   *canvas.FontFamily
	HeaderFont   *canvas.FontFace
	SceneWidth   float64
	ImageHeight  float64
	ImageSpacing float64
	LineSpacing  float64
}

type Section struct {
	photos   []*Photo
	Inverted bool
}

type SectionPhoto struct {
	Index int
	Photo *Photo
	Size  Size
}

type LayoutPhoto struct {
	Index int
	Photo Photo
	Info  ImageInfo
}

type PhotoRegionSource struct {
	imageSource *storage.ImageSource
}

type RegionThumbnail struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type PhotoRegionData struct {
	Id         int               `json:"id"`
	Path       string            `json:"path"`
	Filename   string            `json:"filename"`
	Extension  string            `json:"extension"`
	Video      bool              `json:"video"`
	Width      int               `json:"width"`
	Height     int               `json:"height"`
	Thumbnails []RegionThumbnail `json:"thumbnails"`
	// SmallestThumbnail     string   `json:"smallest_thumbnail"`
}

func (regionSource PhotoRegionSource) getRegionFromPhoto(id int, photo *Photo, scene *Scene, regionConfig RegionConfig) Region {

	source := regionSource.imageSource

	originalPath := photo.GetPath(source)

	var thumbnails []RegionThumbnail
	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
		thumbnailPath := thumbnail.GetPath(originalPath)
		if source.Exists(thumbnailPath) {
			thumbnails = append(thumbnails, RegionThumbnail{
				Name:   thumbnail.Name,
				Width:  thumbnail.Size.X,
				Height: thumbnail.Size.Y,
			})
		}
	}

	size := photo.GetSize(source)

	return Region{
		Id:     id,
		Bounds: photo.Sprite.Rect,
		Data: PhotoRegionData{
			Id:         int(photo.Id),
			Path:       originalPath,
			Filename:   filepath.Base(originalPath),
			Extension:  strings.ToLower(filepath.Ext(originalPath)),
			Video:      source.IsSupportedVideo(originalPath),
			Width:      size.X,
			Height:     size.Y,
			Thumbnails: thumbnails,
		},
	}
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect Rect, scene *Scene, regionConfig RegionConfig) []Region {
	regions := make([]Region, 0)
	photos := scene.GetVisiblePhotos(rect, regionConfig.Limit)
	for photo := range photos {
		regions = append(regions, regionSource.getRegionFromPhoto(
			photo.Index,
			photo.Photo,
			scene, regionConfig,
		))
	}
	return regions
}

func (regionSource PhotoRegionSource) GetRegionById(id int, scene *Scene, regionConfig RegionConfig) Region {
	if id < 0 || id >= len(scene.Photos)-1 {
		return Region{Id: -1}
	}
	photo := scene.Photos[id]
	return regionSource.getRegionFromPhoto(id, &photo, scene, regionConfig)
}

func layoutFitRow(row []SectionPhoto, bounds Rect, imageSpacing float64) float64 {
	count := len(row)
	if count == 0 {
		return 1.
	}
	firstPhoto := row[0]
	firstRect := firstPhoto.Photo.Sprite.Rect
	lastPhoto := row[count-1]
	lastRect := lastPhoto.Photo.Sprite.Rect
	totalSpacing := float64(count-1) * imageSpacing

	rowWidth := lastRect.X + lastRect.W
	scale := (bounds.W - totalSpacing) / (rowWidth - totalSpacing)
	x := firstRect.X
	for i := range row {
		photo := row[i]
		rect := photo.Photo.Sprite.Rect
		photo.Photo.Sprite.Rect = Rect{
			X: x,
			Y: rect.Y,
			W: rect.W * scale,
			H: rect.H * scale,
		}
		x += photo.Photo.Sprite.Rect.W + imageSpacing
	}

	// fmt.Printf("fit row width %5.2f / %5.2f -> %5.2f  scale %.2f\n", rowWidth, bounds.W, lastPhoto.Photo.Original.Sprite.Rect.X+lastPhoto.Photo.Original.Sprite.Rect.W, scale)

	x -= imageSpacing
	return scale
}

func orderSectionPhotoStream(section *Section, input chan SectionPhoto, output chan SectionPhoto) {
	var buffer []SectionPhoto
	index := 0
	if section.Inverted {
		index = len(section.photos) - 1
	}
	for photo := range input {

		if photo.Index != index {
			buffer = append(buffer, photo)
			// log.Println("buffer", len(buffer))
			continue
		}

		// log.Println("order", index, photo.Index)
		output <- photo
		if section.Inverted {
			index--
		} else {
			index++
		}

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
					if section.Inverted {
						index--
					} else {
						index++
					}
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
		size := photo.GetSize(source)
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
	go orderSectionPhotoStream(section, unordered, output)

	if section.Inverted {
		for i := range section.photos {
			index <- (len(section.photos) - 1 - i)
		}
	} else {
		for i := range section.photos {
			index <- i
		}
	}

	close(index)
	wg.Wait()
	close(unordered)
}

func layoutSectionPhotos(photos chan SectionPhoto, bounds Rect, boundsOut chan Rect, config LayoutConfig, scene *Scene, source *storage.ImageSource) {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	i := 0

	row := make([]SectionPhoto, 0)

	for photo := range photos {

		// log.Println("layout", photo.Index)

		aspectRatio := float64(photo.Size.X) / float64(photo.Size.Y)
		imageWidth := float64(config.ImageHeight) * aspectRatio

		if x+imageWidth > bounds.W {
			scale := layoutFitRow(row, bounds, config.ImageSpacing)
			row = nil
			x = 0
			y += config.ImageHeight*scale + config.LineSpacing
		}

		// fmt.Printf("%4.0f %4.0f %4.0f %4.0f %4.0f %4.0f %4.0f\n", bounds.X, bounds.Y, x, y, imageHeight, photo.Size.Width, photo.Size.Height)

		photo.Photo.Sprite.PlaceFitHeight(
			bounds.X+x,
			bounds.Y+y,
			config.ImageHeight,
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

		x += imageWidth + config.ImageSpacing

		now := time.Now()
		if now.Sub(lastLogTime) > 1*time.Second {
			lastLogTime = now
			log.Printf("layout section %d\n", photo.Index)
		}
		i++
	}
	x = 0
	y += config.ImageHeight + config.LineSpacing
	boundsOut <- Rect{
		X: bounds.X,
		Y: bounds.Y,
		W: bounds.W,
		H: y,
	}
	close(boundsOut)
}

func layoutSectionList(section *Section, bounds Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, source *storage.ImageSource) canvas.Point {
	x := 0.
	y := 0.
	lastLogTime := time.Now()
	photoCount := len(section.photos)
	for i := range section.photos {
		photo := section.photos[i]
		size := photo.GetSize(source)

		aspectRatio := float64(size.X) / float64(size.Y)
		imageWidth := float64(imageHeight) * aspectRatio

		if x+imageWidth+imageSpacing > bounds.W {
			x = 0
			y += imageHeight + lineSpacing
		}

		photo.Sprite.PlaceFitHeight(
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

func getLayoutPhotosUnordered(id int, photos []Photo, indices chan int, output chan LayoutPhoto, wg *sync.WaitGroup, source *storage.ImageSource) {
	for i := range indices {
		photo := &photos[i]
		path := photo.GetPath(source)
		info := source.GetImageInfo(path)
		output <- LayoutPhoto{
			Index: i,
			Photo: *photo,
			Info:  info,
		}
	}
	wg.Done()
}

func getLayoutPhotoChan(photos []Photo, source *storage.ImageSource) <-chan LayoutPhoto {
	finished := ElapsedWithCount("layout load info", len(photos))

	indices := make(chan int)
	layoutPhotos := make(chan LayoutPhoto, 10)

	concurrent := 20

	wg := &sync.WaitGroup{}
	wg.Add(concurrent)
	for i := 0; i < concurrent; i++ {
		go getLayoutPhotosUnordered(i, photos, indices, layoutPhotos, wg, source)
	}

	go func() {
		for i := range photos {
			indices <- i
		}
		close(indices)
		wg.Wait()
		finished()
		close(layoutPhotos)
	}()

	// sort.Slice(scene.Photos, func(i, j int) bool {
	// 	a := source.GetImageInfo(scene.Photos[i].Original.Path)
	// 	b := source.GetImageInfo(scene.Photos[j].Original.Path)
	// 	return a.DateTime.After(b.DateTime)
	// })

	return layoutPhotos
}

func layoutPhotoChanToSlice(input <-chan LayoutPhoto) []LayoutPhoto {
	var layoutPhotos []LayoutPhoto
	lastIndex := -1
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	for photo := range input {
		now := time.Now()
		if now.Sub(lastLogTime) > logInterval {
			perSec := float64(photo.Index-lastIndex) / logInterval.Seconds()
			log.Printf("layout load info %d, %.2f / sec\n", photo.Index, perSec)
			lastLogTime = now
			lastIndex = photo.Index
		}
		layoutPhotos = append(layoutPhotos, photo)
	}
	return layoutPhotos
}

func getLayoutPhotos(photos []Photo, source *storage.ImageSource) []LayoutPhoto {
	return layoutPhotoChanToSlice(getLayoutPhotoChan(photos, source))
}

func sortNewestToOldest(photos []LayoutPhoto) {
	defer ElapsedWithCount("layout sort", len(photos))()

	sort.Slice(photos, func(i, j int) bool {
		a := photos[i]
		b := photos[j]
		return a.Info.DateTime.After(b.Info.DateTime)
	})
}

func sortOldestToNewest(photos []LayoutPhoto) {
	defer ElapsedWithCount("layout sort", len(photos))()

	sort.Slice(photos, func(i, j int) bool {
		a := photos[i]
		b := photos[j]
		return a.Info.DateTime.Before(b.Info.DateTime)
	})
}
