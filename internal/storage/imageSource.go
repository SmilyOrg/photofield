package photofield

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"

	. "photofield/internal"
	decoder "photofield/internal/decoder"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/dgraph-io/ristretto"
	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
)

type ImageInfoDb struct {
	Path      string `gorm:"type:varchar(4096);primary_key;unique_index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	ImageInfo
}

type ImageSource struct {
	Thumbnails []Thumbnail

	imagesLoading  sync.Map
	images         *ristretto.Cache
	infos          *ristretto.Cache
	fileExists     *ristretto.Cache
	db             *gorm.DB
	dbPendingInfos chan *ImageInfoDb
	// imageByPath       sync.Map
	// imageConfigByPath sync.Map
}

type imageRef struct {
	path  string
	image *image.Image
	// mutex LoadingImage
	// LoadingImage.loadOnce
}

type loadingImage struct {
	imageRef *imageRef
	mutex    sync.RWMutex
	// mutex sync.Mutex
	// cond  *sync.Cond
}

func NewImageSource() *ImageSource {
	var err error
	source := ImageSource{}
	source.images, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache
		// MaxCost:     1 << 27, // maximum cost of cache
		BufferItems: 64, // number of keys per Get buffer.
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			imageRef := value.(imageRef)
			// config := source.GetImageConfig(imageRef.path)
			// switch imageType := (*imageRef.image).(type) {
			// case image.YCbCr:
			// 	println("YCbCr")
			// default:
			// 	println("UNKNOWN")
			// }
			// return 1
			switch img := (*imageRef.image).(type) {

			case *image.YCbCr:
				return int64(unsafe.Sizeof(*img)) +
					int64(cap(img.Y))*int64(unsafe.Sizeof(img.Y[0])) +
					int64(cap(img.Cb))*int64(unsafe.Sizeof(img.Cb[0])) +
					int64(cap(img.Cr))*int64(unsafe.Sizeof(img.Cr[0]))

			case *image.Gray:
				return int64(unsafe.Sizeof(*img)) +
					int64(cap(img.Pix))*int64(unsafe.Sizeof(img.Pix[0]))

			case *image.NRGBA:
				return int64(unsafe.Sizeof(*img)) +
					int64(cap(img.Pix))*int64(unsafe.Sizeof(img.Pix[0]))

			case *image.RGBA:
				return int64(unsafe.Sizeof(*img)) +
					int64(cap(img.Pix))*int64(unsafe.Sizeof(img.Pix[0]))

			default:
				panic(fmt.Sprintf("Unable to compute cost, unsupported image format %v", reflect.TypeOf(img).String()))
			}
			// ycbcr, ok := (*imageRef.image).(*image.YCbCr)
			// if !ok {
			// 	// fmt.Println("Unable to compute cost, unsupported image format")
			// 	// return 1
			// 	panic("Unable to compute cost, unsupported image format")
			// }
			// // fmt.Printf("%s %d %d %d %d %d\n", imageRef.path, unsafe.Sizeof(*ycbcr), unsafe.Sizeof(ycbcr.Y[0]), cap(ycbcr.Y), cap(ycbcr.Cb), cap(ycbcr.Cr))
			// bytes := int64(unsafe.Sizeof(*ycbcr)) +
			// 	int64(cap(ycbcr.Y))*int64(unsafe.Sizeof(ycbcr.Y[0])) +
			// 	int64(cap(ycbcr.Cb))*int64(unsafe.Sizeof(ycbcr.Cb[0])) +
			// 	int64(cap(ycbcr.Cr))*int64(unsafe.Sizeof(ycbcr.Cr[0]))
			// fmt.Printf("%s %d\n", imageRef.path, bytes)
			// return bytes
			// return unsafe.Sizeof(image.RGBA) + config.Width*config.Height
		},
	})
	if err != nil {
		panic(err)
	}

	source.infos, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 27, // maximum cost of cache (128MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}

	source.fileExists, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 27, // maximum cost of cache (128MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}

	db, err := gorm.Open("sqlite3", "photofield.cache.db")
	if err != nil {
		panic("failed to connect database")
	}
	source.db = db

	// Migrate the schema
	db.AutoMigrate(&ImageInfoDb{})

	source.dbPendingInfos = make(chan *ImageInfoDb)
	go writePendingInfos(source.dbPendingInfos, db)

	// // Create
	// db.Create(&Product{Code: "L1212", Price: 1000})

	// // Read
	// var product Product
	// db.First(&product, 1)                   // find product with id 1
	// db.First(&product, "code = ?", "L1212") // find product with code l1212

	// // Update - update product's price to 2000
	// db.Model(&product).Update("Price", 2000)

	return &source
}

func (source *ImageSource) ListImages(dir string, maxPhotos int, paths chan string, wg *sync.WaitGroup) error {
	lastLogTime := time.Now()
	files := 0
	error := godirwalk.Walk(dir, &godirwalk.Options{
		Unsorted: true,
		Callback: func(path string, dir *godirwalk.Dirent) error {
			if strings.Contains(path, "@eaDir") {
				return filepath.SkipDir
			}
			if !strings.HasSuffix(strings.ToLower(path), ".jpg") {
				return nil
			}
			files++
			now := time.Now()
			if now.Sub(lastLogTime) > 1*time.Second {
				lastLogTime = now
				log.Printf("listing %d\n", files)
			}
			paths <- path
			if files >= maxPhotos {
				return errors.New("Skipping the rest")
			}
			return nil
		},
	})
	wg.Done()
	return error
}

func (source *ImageSource) LoadImage(path string) (*image.Image, error) {
	// fmt.Printf("loading %s\n", path)
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	image, _, err := decoder.Decode(file)
	return &image, err
}

func (source *ImageSource) GetSmallestThumbnail(path string) string {
	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
		thumbnailPath := thumbnail.GetPath(path)
		if source.Exists(thumbnailPath) {
			return thumbnailPath
		}
	}
	return ""
}

func (source *ImageSource) LoadImageColor(path string) (color.RGBA, error) {
	colorPath := source.GetSmallestThumbnail(path)
	if colorPath == "" {
		colorPath = path
	}
	colorImage, err := source.LoadImage(colorPath)
	if err != nil {
		return color.RGBA{}, err
	}
	centroids, err := prominentcolor.Kmeans(*colorImage)
	if err != nil {
		return color.RGBA{}, err
	}
	promColor := centroids[0]
	return color.RGBA{
		A: 0xFF,
		R: uint8(promColor.Color.R),
		G: uint8(promColor.Color.G),
		B: uint8(promColor.Color.B),
	}, nil
}

func (source *ImageSource) LoadImageInfo(path string) (*ImageInfo, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}

	var info ImageInfo
	err = decoder.DecodeInfo(file, &info)
	if err != nil {
		return nil, err
	}

	color, err := source.LoadImageColor(path)
	if err != nil {
		return &info, err
	}

	info.SetColorRGBA(color)
	return &info, nil
}

func (source *ImageSource) CacheImage(path string) (*image.Image, error) {
	image, err := source.LoadImage(path)
	source.images.Set(path, imageRef{
		path:  path,
		image: image,
	}, 0)
	return image, err
}

func (source *ImageSource) Exists(path string) bool {
	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
	// 	source.fileExists.Metrics.Ratio()*100,
	// 	source.fileExists.Metrics.CostAdded()/1024/1024,
	// 	source.fileExists.Metrics.CostEvicted()/1024/1024,
	// 	source.fileExists.Metrics.Hits(),
	// 	source.fileExists.Metrics.Misses())

	value, found := source.fileExists.Get(path)
	if found {
		return value.(bool)
	}
	// statStart := time.Now()
	_, err := os.Stat(path)
	// statElapsed := time.Since(statStart)
	// if statElapsed > 100*time.Millisecond {
	// 	log.Printf("Stat took %s for %s\n", statElapsed, path)
	// }

	exists := !os.IsNotExist(err)
	source.fileExists.SetWithTTL(path, exists, 1, 1*time.Minute)
	return exists
}

func (source *ImageSource) GetImage(path string) (*image.Image, error) {
	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
	// 	source.images.Metrics.Ratio()*100,
	// 	source.images.Metrics.CostAdded()/1024/1024,
	// 	source.images.Metrics.CostEvicted()/1024/1024,
	// 	source.images.Metrics.Hits(),
	// 	source.images.Metrics.Misses())
	tries := 1000
	for try := 0; try < tries; try++ {
		value, found := source.images.Get(path)
		if found {
			return value.(imageRef).image, nil
		} else {
			loading := &loadingImage{}
			// loadingImage.cond = sync.NewCond(&loadingImage.mutex)
			loading.mutex.Lock()
			stored, loaded := source.imagesLoading.LoadOrStore(path, loading)
			if loaded {
				// loadingImage.mutex.Unlock()
				// loadingImage = stored.(*LoadingImage)
				// loadingImage.mutex.Lock()
				// if loadingImage.cond != nil {
				// 	log.Printf("%v not found, try %v, waiting\n", path, try)
				// 	loadingImage.cond.Wait()
				// 	log.Printf("%v not found, try %v, waiting done\n", path, try)
				// 	imageRef := loadingImage.imageRef
				// 	loadingImage.mutex.Unlock()
				// 	return imageRef.image, nil
				// } else {
				// 	log.Printf("%v not found, try %v, done (no cond)\n", path, try)
				// 	return loadingImage.imageRef.image, nil
				// }
				loading.mutex.Unlock()
				loading = stored.(*loadingImage)
				// log.Printf("%v not found, try %v, waiting load, mutex rlocked\n", path, try)
				loading.mutex.RLock()
				// log.Printf("%v not found, try %v, waiting done, mutex runlocked\n", path, try)
				imageRef := loading.imageRef
				loading.mutex.RUnlock()
				if imageRef == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				return imageRef.image, nil
			} else {

				// log.Printf("%v not found, try %v, loading, mutex locked\n", path, try)
				image, err := source.LoadImage(path)
				if err != nil {
					// log.Println("Unable to load image", err)
					return nil, errors.New(fmt.Sprintf("Unable to load image from path: %s", path))
				}

				imageRef := imageRef{
					path:  path,
					image: image,
				}
				source.images.Set(path, imageRef, 0)
				loading.imageRef = &imageRef
				// log.Printf("%v not found, try %v, loaded, broadcast\n", path, try)
				// cond := loadingImage.cond
				// loadingImage.cond = nil
				// cond.Broadcast()

				// source.imagesLoading.Delete(path)
				// log.Printf("%v not found, try %v, loaded, mutex unlocked\n", path, try)
				loading.mutex.Unlock()

				return image, nil
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("Unable to get image after %v tries", tries))

	// imageRef := &ImageRef{}
	// imageRef.mutex.Lock()
	// stored, loaded := source.imageByPath.LoadOrStore(path, imageRef)

	// var loadedImage *image.Image

	// if loaded {
	// 	imageRef.mutex.Unlock()
	// 	imageRef = stored.(*ImageRef)
	// 	imageRef.mutex.RLock()
	// 	loadedImage = imageRef.image
	// 	imageRef.mutex.RUnlock()
	// } else {
	// 	image, err := source.LoadImage(path)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	imageRef.image = image
	// 	loadedImage = imageRef.image
	// 	imageRef.mutex.Unlock()
	// }

	// return loadedImage
}

func writePendingInfos(pendingInfos chan *ImageInfoDb, db *gorm.DB) {
	for imageInfo := range pendingInfos {
		db.Where("path = ?", imageInfo.Path).
			Assign(imageInfo).
			Assign(imageInfo.ImageInfo).
			FirstOrCreate(imageInfo)
	}
}

func (source *ImageSource) GetImageInfo(path string) *ImageInfo {
	value, found := source.infos.Get(path)
	if found {
		return value.(*ImageInfo)
	} else {
		var imageInfoDb ImageInfoDb
		source.db.First(&imageInfoDb, "path = ?", path)

		valid := true
		if imageInfoDb.Path == "" {
			valid = false
		}
		// if imageInfoDb.ImageInfo.Width == 0 || imageInfoDb.ImageInfo.Height == 0 {
		// 	valid = false
		// }
		// if imageInfoDb.ImageInfo.DateTime.IsZero() {
		// 	valid = false
		// }
		// if imageInfoDb.ImageInfo.Color == 0 {
		// 	valid = false
		// }
		if valid {
			return &imageInfoDb.ImageInfo
		}

		info, err := source.LoadImageInfo(path)
		if err != nil {
			return &ImageInfo{}
		}
		source.infos.Set(path, info, 1)

		source.dbPendingInfos <- &ImageInfoDb{
			Path:      path,
			ImageInfo: *info,
		}

		return info
	}
}

// func (source *ImageSource) GetImageColor(path string, getPathForColor func() string) color.RGBA {
// 	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
// 	// 	source.infos.Metrics.Ratio()*100,
// 	// 	source.infos.Metrics.CostAdded()/1024/1024,
// 	// 	source.infos.Metrics.CostEvicted()/1024/1024,
// 	// 	source.infos.Metrics.Hits(),
// 	// 	source.infos.Metrics.Misses())

// 	value, found := source.infos.Get(path)
// 	info := ImageInfo{}
// 	c := color.RGBA{}
// 	cache := false
// 	save := false
// 	if found {
// 		info := value.(*ImageInfo)
// 		c = info.GetColor()
// 	} else {
// 		infoDb := ImageInfoDb{}
// 		queryStart := time.Now()
// 		source.db.First(&infoDb, "path = ?", path)
// 		queryElapsed := time.Since(queryStart)
// 		if queryElapsed > 100*time.Millisecond {
// 			// log.Printf("GetImageColor query took %s for %s\n", queryElapsed, path)
// 		}
// 		c = infoDb.ImageInfo.GetColor()
// 		cache = true
// 	}
// 	if c.A == 0 {
// 		// println("1")
// 		colorPath := getPathForColor()
// 		if colorPath == "" {
// 			c = color.RGBA{
// 				A: 0xFF,
// 				R: 0xFF,
// 				G: 0x00,
// 				B: 0x00,
// 			}
// 		} else {
// 			colorImage, err := source.LoadImage(colorPath)
// 			if err != nil {
// 				// log.Println("Unable to load image color", err)
// 				c = color.RGBA{
// 					A: 0xFF,
// 					R: 0xFF,
// 					G: 0x00,
// 					B: 0x00,
// 				}
// 			} else {
// 				centroids, err := prominentcolor.Kmeans(*colorImage)
// 				if err == nil {
// 					// panic(err)
// 					promColor := centroids[0]
// 					c = color.RGBA{
// 						A: 0xFF,
// 						R: uint8(promColor.Color.R),
// 						G: uint8(promColor.Color.G),
// 						B: uint8(promColor.Color.B),
// 					}
// 					save = true
// 				}
// 			}
// 		}
// 		cache = true
// 	}
// 	if c.A == 0 {
// 		return color.RGBA{
// 			A: 0xFF,
// 			R: 0x10,
// 			G: 0x10,
// 			B: 0x10,
// 		}
// 	}
// 	if cache || save {
// 		info.SetColorRGBA(c)
// 	}
// 	if cache {
// 		source.infos.Set(path, &info, 1)
// 	}
// 	if save {
// 		infoDb := ImageInfoDb{
// 			Path:      path,
// 			ImageInfo: info,
// 		}
// 		source.dbPendingInfos <- &infoDb
// 	}
// 	return c
// }
