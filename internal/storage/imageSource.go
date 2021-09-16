package photofield

import (
	"embed"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"
	"unsafe"

	. "photofield/internal"
	. "photofield/internal/codec"

	"github.com/EdlinOrg/prominentcolor"
	"github.com/dgraph-io/ristretto"
	"github.com/karrick/godirwalk"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sheerun/queue"
)

var NotAnImageError = errors.New("Not a supported image extension, might be video")
var NotFoundError = errors.New("Not found")
var SkipError = errors.New("Skipping the rest")

type ImageId uint32

type FileConfig struct {
	Extensions []string    `json:"extensions"`
	Thumbnails []Thumbnail `json:"thumbnails"`
}

type ImageSourceConfig struct {
	ListExtensions       []string   `json:"extensions"`
	Images               FileConfig `json:"images"`
	Videos               FileConfig `json:"videos"`
	DateFormats          []string   `json:"date_formats"`
	ConcurrentMetaLoads  int        `json:"concurrent_meta_loads"`
	ConcurrentColorLoads int        `json:"concurrent_color_loads"`
}

type ImageSource struct {
	*ImageSourceConfig

	pathToIndex sync.Map
	paths       []string
	pathMutex   sync.RWMutex

	Coder              *MediaCoder
	imagesLoading      sync.Map
	imagesLoadingCount int
	images             *ristretto.Cache
	infoCache          *ImageInfoSourceCache
	infoDatabase       *ImageInfoSourceSqlite
	fileExists         *ristretto.Cache

	loadQueueMeta  *queue.Queue
	loadQueueColor *queue.Queue
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

func NewImageSource(system System, config ImageSourceConfig, migrations embed.FS) *ImageSource {
	var err error
	source := ImageSource{}
	source.ImageSourceConfig = &config
	source.Coder = NewMediaCoder(system.ExifToolCount)
	source.images, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,                                // number of keys to track frequency of
		MaxCost:     system.Caches.Image.MaxSizeBytes(), // maximum cost of cache
		BufferItems: 64,                                 // number of keys per Get buffer
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			imageRef := value.(imageRef)
			img := imageRef.image
			if img == nil {
				return 1
			}
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

			case *image.CMYK:
				return int64(unsafe.Sizeof(*img)) +
					int64(cap(img.Pix))*int64(unsafe.Sizeof(img.Pix[0]))

			case nil:
				return 1

			default:
				panic(fmt.Sprintf("Unable to compute cost, unsupported image format %v", reflect.TypeOf(img)))
			}
		},
	})
	if err != nil {
		panic(err)
	}
	AddRistrettoMetrics("image_cache", source.images)

	source.infoCache = NewImageInfoSourceCache()
	source.infoDatabase = NewImageInfoSourceSqlite(migrations)

	source.fileExists, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 24, // maximum cost of cache (16MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	AddRistrettoMetrics("file_exists_cache", source.fileExists)

	if system.SkipLoadInfo {
		log.Printf("skipping load info")
	} else {
		source.loadQueueMeta = queue.New()
		source.loadQueueColor = queue.New()
		go source.processQueue(
			"load meta",
			"load_meta",
			source.loadQueueMeta,
			source.loadImageInfosMeta,
			source.ConcurrentMetaLoads,
		)
		go source.processQueue(
			"load color",
			"load_color",
			source.loadQueueColor,
			source.loadImageInfosColor,
			source.ConcurrentColorLoads,
		)
	}

	return &source
}

func (source *ImageSource) Close() {
	source.Coder.Close()
}

func (source *ImageSource) ListImages(dirs []string, maxPhotos int) <-chan string {
	for i := range dirs {
		dirs[i] = filepath.FromSlash(dirs[i])
	}
	return source.infoDatabase.ListPaths(dirs, maxPhotos)
}

func (source *ImageSource) ListImageInfos(dirs []string, options ListOptions) <-chan SourcedImageInfo {
	for i := range dirs {
		dirs[i] = filepath.FromSlash(dirs[i])
	}
	out := make(chan SourcedImageInfo, 10000)
	go func() {
		infos := source.infoDatabase.List(dirs, options)
		for info := range infos {
			if info.NeedsMeta() || info.NeedsColor() {
				info.ImageInfo = source.GetImageInfo(info.Path)
			}
			out <- info.SourcedImageInfo
		}
		close(out)
	}()
	return out
}

func (source *ImageSource) IndexImages(dir string, maxPhotos int, counter chan<- int) {
	dir = filepath.FromSlash(dir)
	indexed := make(map[string]struct{})
	for path := range source.walkImages(dir, maxPhotos) {
		source.infoDatabase.Write(path, ImageInfo{}, AppendPath)
		indexed[path] = struct{}{}
		// Uncomment to test slow indexing
		// time.Sleep(10 * time.Millisecond)
		counter <- 1
	}
	source.infoDatabase.DeleteNonexistent(dir, indexed)
}

func (source *ImageSource) QueueMetaLoads(ids <-chan ImageId) {
	if source.loadQueueMeta != nil {
		for id := range ids {
			source.loadQueueMeta.Append(id)
		}
	}
}

func (source *ImageSource) QueueColorLoads(ids <-chan ImageId) {
	if source.loadQueueColor != nil {
		for id := range ids {
			source.loadQueueColor.Append(id)
		}
	}
}

func (source *ImageSource) walkImages(dir string, maxPhotos int) <-chan string {
	out := make(chan string)
	go func() {
		finished := Elapsed(fmt.Sprintf("index %s", dir))
		defer finished()

		lastLogTime := time.Now()
		files := 0
		err := godirwalk.Walk(dir, &godirwalk.Options{
			Unsorted: true,
			Callback: func(path string, walk_dir *godirwalk.Dirent) error {
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
					log.Printf("indexing %s %d files\n", dir, files)
				}
				out <- path
				if maxPhotos > 0 && files >= maxPhotos {
					return SkipError
				}
				return nil
			},
		})
		if err != nil && err != SkipError {
			log.Printf("Error indexing files: %s\n", err.Error())
		}

		close(out)
	}()
	return out
}

func (source *ImageSource) decode(path string, reader io.ReadSeeker) (*image.Image, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, "jpg") || strings.HasSuffix(lower, "jpeg") {
		image, err := source.Coder.DecodeJpeg(reader)
		return &image, err
	}

	image, _, err := source.Coder.Decode(reader)
	return &image, err
}

func (source *ImageSource) LoadImage(path string) (*image.Image, error) {
	// fmt.Printf("loading %s\n", path)
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	return source.decode(path, file)
}

func (source *ImageSource) GetSmallestThumbnail(path string) string {
	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
		thumbnailPath := thumbnail.GetPath(path)
		if source.Exists(thumbnailPath) {
			return thumbnailPath
		}
	}
	return ""
}

func (source *ImageSource) LoadSmallestImage(path string) (*image.Image, error) {
	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
		thumbnailPath := thumbnail.GetPath(path)
		file, err := os.Open(thumbnailPath)
		if err != nil {
			continue
		}
		defer file.Close()
		return source.decode(thumbnailPath, file)
	}
	return source.LoadImage(path)
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
	err := source.Coder.DecodeInfo(path, &info)
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

func (source *ImageSource) LoadImageInfoMeta(path string) (ImageInfo, error) {
	var info ImageInfo
	err := source.Coder.DecodeInfo(path, &info)
	if err != nil {
		return info, err
	}
	return info, nil
}

func (source *ImageSource) LoadImageInfoColor(path string) (ImageInfo, error) {
	var info ImageInfo
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

	for _, format := range source.DateFormats {
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
	value, found := source.fileExists.Get(path)
	if found {
		return value.(bool)
	}
	_, err := os.Stat(path)

	exists := !os.IsNotExist(err)
	source.fileExists.SetWithTTL(path, exists, 1, 1*time.Minute)
	return exists
}

func (source *ImageSource) IsSupportedImage(path string) bool {
	supportedImage := false
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.Images.Extensions {
		if pathExt == ext {
			supportedImage = true
			break
		}
	}
	return supportedImage
}

func (source *ImageSource) IsSupportedVideo(path string) bool {
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.Videos.Extensions {
		if pathExt == ext {
			return true
		}
	}
	return false
}

func (source *ImageSource) GetImagePath(id ImageId) (string, error) {
	index := int(id) - 1
	source.pathMutex.RLock()
	if index < 0 || index >= len(source.paths) {
		return "", NotFoundError
	}
	path := source.paths[index]
	source.pathMutex.RUnlock()
	return path, nil
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

func (source *ImageSource) processQueue(name string, id string, queue *queue.Queue, workerFn func(<-chan string), workerCount int) {

	loadCount := 0
	lastLoadCount := 0
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	AddQueueMetrics(id, queue)
	var doneCounter = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: MetricsNamespace,
		Name:      id + "_done",
	})

	logging := false

	paths := make(chan string)
	defer close(paths)

	for i := 0; i < workerCount; i++ {
		go workerFn(paths)
	}

	for {
		id := queue.Pop().(ImageId)
		if id == 0 {
			log.Printf("%s queue stopping\n", name)
			return
		}

		path, err := source.GetImagePath(id)
		if err != nil {
			panic("Unable to load image info for non-existing image id")
		}
		paths <- path
		doneCounter.Inc()

		now := time.Now()
		elapsed := now.Sub(lastLogTime)
		if elapsed > logInterval || queue.Length() == 0 {
			perSec := float64(loadCount-lastLoadCount) / elapsed.Seconds()
			pendingCount := queue.Length()
			percent := 100
			if loadCount+pendingCount > 0 {
				percent = loadCount * 100 / (loadCount + pendingCount)
			}
			log.Printf("%s %4d%% completed, %5d loaded, %5d pending, %.2f / sec\n", name, percent, loadCount, pendingCount, perSec)
			lastLoadCount = loadCount
			lastLogTime = now
		}

		loadCount++

		if logging {
			// log.Printf("image info load for id %5d, %5d pending, %5d ms get file, %5d ms set db, %5d ms set cache\n", id, len(backlog), fileGetMs, dbSetMs, cacheSetMs)
			log.Printf("%s queue id %5d, %5d pending\n", name, id, queue.Length())
		}
	}
}

func (source *ImageSource) loadImageInfosMeta(paths <-chan string) {
	for path := range paths {
		info, err := source.LoadImageInfoMeta(path)
		if err != nil {
			fmt.Println("Unable to load image info meta", err, path)
		}
		source.infoDatabase.Write(path, info, UpdateMeta)
		source.infoCache.Delete(path)
	}
}

func (source *ImageSource) loadImageInfosColor(paths <-chan string) {
	for path := range paths {
		info, err := source.LoadImageInfoColor(path)
		if err != nil {
			fmt.Println("Unable to load image info color", err, path)
		}
		source.infoDatabase.Write(path, info, UpdateColor)
		source.infoCache.Delete(path)
	}
}

func (source *ImageSource) GetImageInfo(path string) ImageInfo {
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
	result, found := source.infoDatabase.Get(path)
	info = result.ImageInfo
	dbGetMs := time.Since(startTime).Milliseconds()
	needsMeta := result.NeedsMeta()
	if found && !needsMeta {
		startTime = time.Now()
		source.infoCache.Set(path, info)
		cacheSetMs := time.Since(startTime).Milliseconds()
		if logging {
			log.Printf("image info %5d ms get cache, %5d ms get db, %5d ms set cache\n", cacheGetMs, dbGetMs, cacheSetMs)
		}
	}

	startTime = time.Now()
	needsColor := result.NeedsColor()
	if needsMeta || needsColor {
		id := source.GetImageId(path)
		if needsMeta {
			if source.loadQueueMeta != nil {
				source.loadQueueMeta.Append(id)
			}
		}
		if needsColor {
			if source.loadQueueColor != nil {
				source.loadQueueColor.Append(id)
			}
		}
	}
	addPendingMs := time.Since(startTime).Milliseconds()

	if found && !needsMeta {
		return info
	}

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
