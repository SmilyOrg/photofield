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
	"github.com/karrick/godirwalk"
)

var NotAnImageError = errors.New("Not a supported image extension, might be video")

type ImageId uint32

type ImageSource struct {
	ListExtensions  []string
	ImageExtensions []string
	VideoExtensions []string
	Thumbnails      []Thumbnail
	Videos          []Thumbnail

	dateFormats []string

	pathToIndex sync.Map
	paths       []string
	pathMutex   sync.RWMutex

	decoder            *MediaDecoder
	imagesLoading      sync.Map
	imagesLoadingCount int
	images             *ristretto.Cache
	infoCache          *ImageInfoSourceCache
	infoDatabase       *ImageInfoSourceSqlite
	fileExists         *ristretto.Cache

	pendingImageInfoIds chan ImageId

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
	imageRef imageRef
	loaded   chan struct{}
}

func NewImageSource() *ImageSource {
	var err error
	source := ImageSource{}
	source.decoder = NewMediaDecoder(20)
	source.ListExtensions = []string{".jpg"}
	// source.ListExtensions = []string{".jpg", ".mp4"}
	// source.ListExtensions = []string{".mp4"}
	source.ImageExtensions = []string{".jpg", ".jpeg", ".png", ".gif"}
	source.VideoExtensions = []string{".mp4"}
	source.images, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6, // number of keys to track frequency of (1M).
		// MaxCost:     1 << 30, // maximum cost of cache
		MaxCost:     1 << 27, // maximum cost of cache
		BufferItems: 64,      // number of keys per Get buffer.
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

	source.infoCache = NewImageInfoSourceCache()
	source.infoDatabase = NewImageInfoSourceSqlite()

	source.fileExists, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7, // number of keys to track frequency of (10M).
		// MaxCost:     1 << 27, // maximum cost of cache (128MB).
		MaxCost:     1 << 24, // maximum cost of cache (16MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}

	source.dateFormats = []string{
		"20060201_150405",
	}

	source.pendingImageInfoIds = make(chan ImageId, 10000)

	go source.loadPendingImageInfos()

	return &source
}

func (source *ImageSource) GetMetrics() ImageSourceMetrics {
	return ImageSourceMetrics{
		Cache: ImageSourceMetricsCaches{
			Images: source.GetCacheMetrics(source.images),
			// Infos:  source.GetCacheMetrics(source.infos),
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
			// Duplicate photos for testing
			// for i := 0; i < 50-1; i++ {
			// 	paths <- path
			// }
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

func (source *ImageSource) LoadSmallestImage(path string) (*image.Image, error) {
	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
		thumbnailPath := thumbnail.GetPath(path)
		file, err := os.Open(thumbnailPath)
		if err != nil {
			continue
		}
		defer file.Close()
		image, _, err := source.decoder.Decode(file)
		return &image, err
	}
	image, err := source.LoadImage(path)
	return image, err
}

func (source *ImageSource) LoadImageColor(path string) (color.RGBA, error) {
	colorImage, err := source.LoadSmallestImage(path)
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

func (source *ImageSource) LoadImageInfo(path string) (ImageInfo, error) {
	var info ImageInfo
	err := source.decoder.DecodeInfo(path, &info)
	if err != nil {
		return info, err
	}

	color, err := source.LoadImageColor(path)
	if err != nil {
		return info, err
	}
	info.SetColorRGBA(color)

	return info, nil
}

func (source *ImageSource) LoadImageInfoHeuristic(path string) (ImageInfo, error) {
	var info ImageInfo

	info.Width = 4000
	info.Height = 3000
	info.Color = 0xFFE8EAED

	baseName := filepath.Base(path)
	name := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	for _, format := range source.dateFormats {
		date, err := time.Parse(format, name)
		if err == nil {
			info.DateTime = date
			break
		}
	}

	if info.DateTime.IsZero() {
		fileInfo, err := os.Stat(path)
		if err == nil {
			info.DateTime = fileInfo.ModTime()
		}
	}

	return info, nil
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
	index := int(id) - 1
	source.pathMutex.RLock()
	if index < 0 || index >= len(source.paths) {
		log.Printf("Unable to get image path, id not found: %v\n", index)
		panic("Unable to get image path")
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
	count := 1 + len(source.paths)
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
	// fmt.Printf("images %3.0f%% hit ratio, cached %3d MiB, loading %3d, added %d MiB, evicted %d MiB, hits %d, misses %d\n",
	// 	source.images.Metrics.Ratio()*100,
	// 	(source.images.Metrics.CostAdded()-source.images.Metrics.CostEvicted())/1024/1024,
	// 	source.imagesLoadingCount,
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
			// loading.mutex.Lock()
			loading.loaded = make(chan struct{})
			stored, loaded := source.imagesLoading.LoadOrStore(path, loading)
			if loaded {
				loading = stored.(*loadingImage)
				// log.Printf("%v blocking on channel, try %v\n", path, try)
				<-loading.loaded
				// log.Printf("%v channel unblocked\n", path)
				imageRef := loading.imageRef
				return imageRef.image, imageRef.err
			} else {

				source.imagesLoadingCount++

				// log.Printf("%v not found, try %v, loading, mutex locked\n", path, try)

				// log.Printf("%v loading, try %v\n", path, try)
				image, err := source.LoadImage(path)
				imageRef := imageRef{
					path:  path,
					image: image,
					err:   err,
				}
				source.images.Set(path, imageRef, 0)
				loading.imageRef = imageRef

				// log.Printf("%v loaded, closing channel\n", path)
				close(loading.loaded)

				source.imagesLoading.Delete(path)
				source.imagesLoadingCount--
				// log.Printf("%v loaded, map entry deleted\n", path)

				return image, err
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("Unable to get image after %v tries", tries))
}

func (source *ImageSource) loadPendingImageInfos() {

	loadCount := 0
	lastLoadCount := 0
	lastLogTime := time.Now()
	logInterval := 2 * time.Second

	logging := false

	backlog := make([]ImageId, 0)
	paths := make(chan string)
	defer close(paths)

	for i := 0; i < 4; i++ {
		go source.loadImageInfos(paths)
	}

	for {
		select {
		case id := <-source.pendingImageInfoIds:
			if id == 0 {
				println("stopping pending image info load")
				return
			}
			backlog = append(backlog, id)
		default:

			if len(backlog) == 0 {
				time.Sleep(1 * time.Second)
				continue
			}

			var id ImageId

			id, backlog = backlog[len(backlog)-1], backlog[:len(backlog)-1]
			path := source.GetImagePath(id)
			paths <- path

			now := time.Now()
			elapsed := now.Sub(lastLogTime)
			if elapsed > logInterval || len(backlog) == 0 {
				perSec := float64(loadCount-lastLoadCount) / elapsed.Seconds()
				pendingCount := len(backlog)
				log.Printf("load info %4d%% completed, %5d loaded, %5d pending, %.2f / sec\n", loadCount*100/(loadCount+pendingCount), loadCount, pendingCount, perSec)
				lastLoadCount = loadCount
				lastLogTime = now
			}

			loadCount++

			if logging {
				// log.Printf("image info load for id %5d, %5d pending, %5d ms get file, %5d ms set db, %5d ms set cache\n", id, len(backlog), fileGetMs, dbSetMs, cacheSetMs)
				log.Printf("image info load for id %5d, %5d pending\n", id, len(backlog))
			}
		}
	}
}

func (source *ImageSource) loadImageInfos(paths <-chan string) {
	for path := range paths {
		info, err := source.LoadImageInfo(path)
		if err != nil {
			fmt.Println("Unable to load image info", err, path)
		}
		source.infoDatabase.Set(path, info)
		source.infoCache.Set(path, info)
	}
}

func (source *ImageSource) addPendingImageInfo(path string) {
	id := source.GetImageId(path)
	source.pendingImageInfoIds <- id
}

func (source *ImageSource) GetImageInfo(path string) ImageInfo {
	// fmt.Printf("image info %3.0f%% hit ratio, cached %3d KiB, added %d KiB, evicted %d KiB, hits %d, misses %d\n",
	// 	source.infoCache.Cache.Metrics.Ratio()*100,
	// 	(source.infoCache.Cache.Metrics.CostAdded()-source.infoCache.Cache.Metrics.CostEvicted())/1024,
	// 	source.infoCache.Cache.Metrics.CostAdded()/1024,
	// 	source.infoCache.Cache.Metrics.CostEvicted()/1024,
	// 	source.infoCache.Cache.Metrics.Hits(),
	// 	source.infoCache.Cache.Metrics.Misses())

	var info ImageInfo
	var err error
	var found bool

	logging := false

	totalStartTime := time.Now()

	startTime := time.Now()
	info, found = source.infoCache.Get(path)
	cacheGetMs := time.Since(startTime).Milliseconds()
	if found {
		// if (logging) log.Printf("image info %5d ms get cache\n", cacheGetMs)
		return info
	}

	startTime = time.Now()
	info, found = source.infoDatabase.Get(path)
	dbGetMs := time.Since(startTime).Milliseconds()
	if found {
		startTime = time.Now()
		source.infoCache.Set(path, info)
		cacheSetMs := time.Since(startTime).Milliseconds()
		if logging {
			log.Printf("image info %5d ms get cache, %5d ms get db, %5d ms set cache\n", cacheGetMs, dbGetMs, cacheSetMs)
		}
		return info
	}

	startTime = time.Now()
	source.addPendingImageInfo(path)
	addPendingMs := time.Since(startTime).Milliseconds()

	startTime = time.Now()
	info, err = source.LoadImageInfoHeuristic(path)
	heuristicGetMs := time.Since(startTime).Milliseconds()
	if err != nil {
		fmt.Println("Unable to load image info heuristic", err, path)
	}

	startTime = time.Now()
	source.infoCache.Set(path, info)
	cacheSetMs := time.Since(startTime).Milliseconds()

	totalMs := time.Since(totalStartTime).Milliseconds()

	logging = totalMs > 1000

	if logging {
		log.Printf("image info %5d ms get cache, %5d ms get db, %5d ms add pending, %5d ms get heuristic, %5d ms set cache\n", cacheGetMs, dbGetMs, addPendingMs, heuristicGetMs, cacheSetMs)
	}
	return info
}
