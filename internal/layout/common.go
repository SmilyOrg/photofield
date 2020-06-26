package photofield

import (
	"log"
	"path/filepath"
	. "photofield/internal"
	. "photofield/internal/display"
	storage "photofield/internal/storage"
	"strings"
	"sync"
	"time"

	"github.com/tdewolff/canvas"
)

type LayoutConfig struct {
	SceneWidth  float64
	ImageHeight float64
	FontFamily  *canvas.FontFamily
	HeaderFont  *canvas.FontFace
}

type Section struct {
	photos []*Photo
}

type SectionPhoto struct {
	Index int
	Photo *Photo
	Size  Size
}

type PhotoRegionSource struct {
	imageSource *storage.ImageSource
}

type PhotoRegionData struct {
	Id        int    `json:"id"`
	Path      string `json:"path"`
	Filename  string `json:"filename"`
	Extension string `json:"extension"`
	Video     bool   `json:"video"`
	// SmallestThumbnail     string   `json:"smallest_thumbnail"`
}

func (regionSource PhotoRegionSource) GetRegionsFromBounds(rect Rect, scene *Scene, regionConfig RegionConfig) []Region {
	regions := make([]Region, 0)
	photos := make(chan PhotoRef)
	source := regionSource.imageSource
	go scene.GetVisiblePhotos(photos, rect, regionConfig.Limit)
	for photo := range photos {
		regions = append(regions, Region{
			Id:     photo.Index,
			Bounds: photo.Photo.Original.Sprite.Rect,
			Data: PhotoRegionData{
				Id:        photo.Index,
				Path:      photo.Photo.Original.Path,
				Filename:  filepath.Base(photo.Photo.Original.Path),
				Extension: strings.ToLower(filepath.Ext(photo.Photo.Original.Path)),
				Video:     source.IsSupportedVideo(photo.Photo.Original.Path),
				// SmallestThumbnail: source.GetSmallestThumbnail(photo.Photo.Original.Path),
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

func layoutSectionPhotos(photos chan SectionPhoto, bounds Rect, boundsOut chan Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, scene *Scene, source *storage.ImageSource) {
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

func layoutSectionList(section *Section, bounds Rect, imageHeight float64, imageSpacing float64, lineSpacing float64, source *storage.ImageSource) canvas.Point {
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
