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
	. "photofield/internal/decoder"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/dgraph-io/ristretto"
	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
)

var NotAnImageError = errors.New("Not a supported image extension, might be video")

type ImageId int

type ImageInfoDb struct {
	Path      string `gorm:"type:varchar(4096);primary_key;unique_index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
	ImageInfo
}

type ImageSource struct {
	ListExtensions  []string
	ImageExtensions []string
	VideoExtensions []string
	Thumbnails      []Thumbnail
	Videos          []Thumbnail

	pathToIndex sync.Map
	paths       []string
	pathMutex   sync.RWMutex

	decoder        *MediaDecoder
	imagesLoading  sync.Map
	images         *ristretto.Cache
	infos          *ristretto.Cache
	fileExists     *ristretto.Cache
	db             *gorm.DB
	dbPendingInfos chan *ImageInfoDb
	// imageByPath       sync.Map
	// imageConfigByPath sync.Map
}

type ImageSourceMetrics struct {
	Cache ImageSourceMetricsCaches `json:"cache"`
}

type ImageSourceMetricsCaches struct {
	Images ImageSourceMetricsCache `json:"images"`
	Infos  ImageSourceMetricsCache `json:"infos"`
	Exists ImageSourceMetricsCache `json:"exists"`
}

type ImageSourceMetricsCache struct {
	HitRatio float64                     `json:"hit_ratio"`
	Hits     uint64                      `json:"hits"`
	Misses   uint64                      `json:"misses"`
	Cost     ImageSourceMetricsCacheCost `json:"cost"`
}

type ImageSourceMetricsCacheCost struct {
	Added   uint64 `json:"added"`
	Evicted uint64 `json:"evicted"`
}

type imageRef struct {
	path  string
	err   error
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
	source.decoder = NewMediaDecoder(5)
	// source.ListExtensions = []string{".jpg"}
	source.ListExtensions = []string{".jpg", ".mp4"}
	// source.ListExtensions = []string{".mp4"}
	source.ImageExtensions = []string{".jpg", ".jpeg", ".png", ".gif"}
	source.VideoExtensions = []string{".mp4"}
	source.images, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache
		// MaxCost:     1 << 27, // maximum cost of cache
		BufferItems: 64, // number of keys per Get buffer.
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			imageRef := value.(imageRef)
			img := imageRef.image
			if img == nil {
				return 1
			}
			// config := source.GetImageConfig(imageRef.path)
			// switch imageType := (*imageRef.image).(type) {
			// case image.YCbCr:
			// 	println("YCbCr")
			// default:
			// 	println("UNKNOWN")
			// }
			// return 1
			switch img := (*img).(type) {

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
				panic(fmt.Sprintf("Unable to compute cost, unsupported image format %v", reflect.TypeOf(img)))
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

func (source *ImageSource) GetMetrics() ImageSourceMetrics {
	return ImageSourceMetrics{
		Cache: ImageSourceMetricsCaches{
			Images: source.GetCacheMetrics(source.images),
			Infos:  source.GetCacheMetrics(source.infos),
			Exists: source.GetCacheMetrics(source.fileExists),
		},
	}
}

func (source *ImageSource) GetCacheMetrics(cache *ristretto.Cache) ImageSourceMetricsCache {
	return ImageSourceMetricsCache{
		HitRatio: cache.Metrics.Ratio(),
		Hits:     cache.Metrics.Hits(),
		Misses:   cache.Metrics.Misses(),
		Cost: ImageSourceMetricsCacheCost{
			Added:   cache.Metrics.CostAdded(),
			Evicted: cache.Metrics.CostEvicted(),
		},
	}
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

			suffix := ""
			for _, ext := range source.ListExtensions {
				if strings.HasSuffix(strings.ToLower(path), ext) {
					suffix = ext
					break
				}
			}
			if suffix == "" {
				return nil
			}

			files++
			now := time.Now()
			if now.Sub(lastLogTime) > 1*time.Second {
				lastLogTime = now
				log.Printf("listing %d\n", files)
			}
			paths <- path
			if maxPhotos > 0 && files >= maxPhotos {
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
	image, _, err := source.decoder.Decode(file)
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
	centroids, err := prominentcolor.KmeansWithAll(1, *colorImage, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, prominentcolor.GetDefaultMasks())
	if err != nil {
		centroids, err = prominentcolor.KmeansWithAll(1, *colorImage, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, make([]prominentcolor.ColorBackgroundMask, 0))
		if err != nil {
			return color.RGBA{}, err
		}
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
	var info ImageInfo
	err := source.decoder.DecodeInfoExifTool(path, &info)
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

func (source *ImageSource) IsSupportedImage(path string) bool {
	supportedImage := false
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.ImageExtensions {
		if pathExt == ext {
			supportedImage = true
			break
		}
	}
	return supportedImage
}

func (source *ImageSource) IsSupportedVideo(path string) bool {
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.VideoExtensions {
		if pathExt == ext {
			return true
		}
	}
	return false
}

func (source *ImageSource) GetImagePath(id ImageId) string {
	index := int(id)
	source.pathMutex.RLock()
	if index < 0 || index >= len(source.paths) {
		log.Printf("Unable to get image path, id not found: %v\n", index)
		return ""
	}
	path := source.paths[index]
	source.pathMutex.RUnlock()
	return path
}

func (source *ImageSource) GetImageId(path string) ImageId {
	source.pathMutex.RLock()
	stored, ok := source.pathToIndex.Load(path)
	if ok {
		source.pathMutex.RUnlock()
		return ImageId(stored.(int))
	}
	source.pathMutex.RUnlock()
	source.pathMutex.Lock()
	count := len(source.paths)
	stored, loaded := source.pathToIndex.LoadOrStore(path, count)
	if loaded {
		source.pathMutex.Unlock()
		return ImageId(stored.(int))
	}
	source.paths = append(source.paths, path)
	// log.Printf("add image id %5d %s\n", stored.(int), path)
	source.pathMutex.Unlock()
	return ImageId(stored.(int))
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
			return value.(imageRef).image, value.(imageRef).err
		} else {
			loading := &loadingImage{}
			loading.mutex.Lock()
			stored, loaded := source.imagesLoading.LoadOrStore(path, loading)
			if loaded {
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
				return imageRef.image, imageRef.err
			} else {

				// log.Printf("%v not found, try %v, loading, mutex locked\n", path, try)
				image, err := source.LoadImage(path)
				imageRef := imageRef{
					path:  path,
					image: image,
					err:   err,
				}
				source.images.Set(path, imageRef, 0)
				loading.imageRef = &imageRef
				loading.mutex.Unlock()

				return image, err
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("Unable to get image after %v tries", tries))
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
	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
	// 	source.infos.Metrics.Ratio()*100,
	// 	source.infos.Metrics.CostAdded()/1024/1024,
	// 	source.infos.Metrics.CostEvicted()/1024/1024,
	// 	source.infos.Metrics.Hits(),
	// 	source.infos.Metrics.Misses())

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
		var info *ImageInfo
		if valid {
			info = &imageInfoDb.ImageInfo
		} else {
			var err error
			info, err = source.LoadImageInfo(path)
			if err != nil {
				// fmt.Println("Unable to load image info", err, path)
				return &ImageInfo{}
			}
			source.dbPendingInfos <- &ImageInfoDb{
				Path:      path,
				ImageInfo: *info,
			}
		}
		source.infos.Set(path, info, 1)
		return info
	}
}
