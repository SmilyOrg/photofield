package image

import (
	"embed"
	"errors"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dgraph-io/ristretto"
	"github.com/docker/go-units"
	"github.com/sheerun/queue"
)

var ErrNotFound = errors.New("not found")
var ErrNotAnImage = errors.New("not a supported image extension, might be video")

type ImageId uint32

type SourcedInfo struct {
	Path string
	Info
}

type CacheConfig struct {
	MaxSize string `json:"max_size"`
}

func (config *CacheConfig) MaxSizeBytes() int64 {
	value, err := units.FromHumanSize(config.MaxSize)
	if err != nil {
		panic(err)
	}
	return value
}

type Caches struct {
	Image CacheConfig
}

type Config struct {
	ExifToolCount        int  `json:"exif_tool_count"`
	SkipLoadInfo         bool `json:"skip_load_info"`
	ConcurrentMetaLoads  int  `json:"concurrent_meta_loads"`
	ConcurrentColorLoads int  `json:"concurrent_color_loads"`

	ListExtensions []string   `json:"extensions"`
	DateFormats    []string   `json:"date_formats"`
	Images         FileConfig `json:"images"`
	Videos         FileConfig `json:"videos"`

	Caches Caches `json:"caches"`
}

type FileConfig struct {
	Extensions []string    `json:"extensions"`
	Thumbnails []Thumbnail `json:"thumbnails"`
}

type Source struct {
	Config

	decoder  *Decoder
	database *Database

	imageInfoCache  InfoCache
	imageCache      *ristretto.Cache
	fileExistsCache *ristretto.Cache

	pathToIndex sync.Map
	paths       []string
	pathMutex   sync.RWMutex

	imagesLoading      sync.Map
	imagesLoadingCount int

	loadQueueMeta  *queue.Queue
	loadQueueColor *queue.Queue
}

func NewSource(config Config, migrations embed.FS) *Source {
	source := Source{}
	source.Config = config
	source.decoder = NewDecoder(config.ExifToolCount)
	source.database = NewDatabase(migrations)
	source.imageInfoCache = newInfoCache()
	source.imageCache = newImageCache(config.Caches)
	source.fileExistsCache = newFileExistsCache()

	if config.SkipLoadInfo {
		log.Printf("skipping load info")
	} else {
		source.loadQueueMeta = queue.New()
		go source.processQueue(
			"load meta",
			"load_meta",
			source.loadQueueMeta,
			source.loadInfosMeta,
			source.ConcurrentMetaLoads,
		)

		source.loadQueueColor = queue.New()
		go source.processQueue(
			"load color",
			"load_color",
			source.loadQueueColor,
			source.loadInfosColor,
			source.ConcurrentColorLoads,
		)
	}

	return &source
}

func (source *Source) Close() {
	source.decoder.Close()
}

func (source *Source) IsSupportedImage(path string) bool {
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

func (source *Source) IsSupportedVideo(path string) bool {
	pathExt := strings.ToLower(filepath.Ext(path))
	for _, ext := range source.Videos.Extensions {
		if pathExt == ext {
			return true
		}
	}
	return false
}

func (source *Source) ListImages(dirs []string, maxPhotos int) <-chan string {
	for i := range dirs {
		dirs[i] = filepath.FromSlash(dirs[i])
	}
	return source.database.ListPaths(dirs, maxPhotos)
}

func (source *Source) ListInfos(dirs []string, options ListOptions) <-chan SourcedInfo {
	for i := range dirs {
		dirs[i] = filepath.FromSlash(dirs[i])
	}
	out := make(chan SourcedInfo, 10000)
	go func() {
		infos := source.database.List(dirs, options)
		for info := range infos {
			if info.NeedsMeta() || info.NeedsColor() {
				info.Info = source.GetInfo(info.Path)
			}
			out <- info.SourcedInfo
		}
		close(out)
	}()
	return out
}

func (source *Source) GetImageId(path string) ImageId {
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

func (source *Source) GetImagePath(id ImageId) (string, error) {
	index := int(id) - 1
	source.pathMutex.RLock()
	if index < 0 || index >= len(source.paths) {
		return "", ErrNotFound
	}
	path := source.paths[index]
	source.pathMutex.RUnlock()
	return path, nil
}

func (source *Source) IndexImages(dir string, maxPhotos int, counter chan<- int) {
	dir = filepath.FromSlash(dir)
	indexed := make(map[string]struct{})
	for path := range walkFiles(dir, source.ListExtensions, maxPhotos) {
		source.database.Write(path, Info{}, AppendPath)
		indexed[path] = struct{}{}
		// Uncomment to test slow indexing
		// time.Sleep(10 * time.Millisecond)
		counter <- 1
	}
	source.database.DeleteNonexistent(dir, indexed)
	source.database.SetIndexed(dir)
	source.database.WaitForCommit()
}

func (source *Source) GetDir(dir string) Info {
	dir = filepath.FromSlash(dir)
	result, _ := source.database.GetDir(dir)
	return result.Info
}
