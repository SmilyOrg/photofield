package image

import (
	"fmt"
	"image"
	"photofield/internal/metrics"
	"reflect"
	"time"
	"unsafe"

	"github.com/dgraph-io/ristretto"
)

type InfoCache struct {
	cache *ristretto.Cache
}

func (c *InfoCache) Get(id ImageId) (Info, bool) {
	value, found := c.cache.Get((uint32)(id))
	if found {
		return value.(Info), true
	}
	return Info{}, false
}

func (c *InfoCache) Set(id ImageId, info Info) error {
	c.cache.Set((uint32)(id), info, (int64)(unsafe.Sizeof(info)))
	return nil
}

func (c *InfoCache) Delete(id ImageId) {
	c.cache.Del((uint32)(id))
}

func newInfoCache() InfoCache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,     // number of keys to track frequency of (1M).
		MaxCost:     1 << 24, // maximum cost of cache (16MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	metrics.AddRistretto("image_info_cache", cache)
	return InfoCache{
		cache: cache,
	}
}

type ImageCache struct {
	cache *ristretto.Cache
}

type ImageLoader interface {
	Acquire(key string, path string, thumbnail *Thumbnail) (image.Image, Info, error)
	Release(key string)
}

func newImageCache(caches Caches) ImageCache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,                         // number of keys to track frequency of
		MaxCost:     caches.Image.MaxSizeBytes(), // maximum cost of cache
		BufferItems: 64,                          // number of keys per Get buffer
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			imageRef := value.(imageRef)
			img := imageRef.image
			if img == nil {
				return 1
			}
			switch img := img.(type) {

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
	metrics.AddRistretto("image_cache", cache)
	return ImageCache{
		cache: cache,
	}
}

func (c *ImageCache) GetOrLoad(path string, thumbnail *Thumbnail, loader ImageLoader) (image.Image, Info, error) {
	key := path
	if thumbnail != nil {
		key += "|thumbnail|" + thumbnail.Name
	}

	value, found := c.cache.Get(key)
	if found {
		imageRef := value.(imageRef)
		return imageRef.image, imageRef.info, imageRef.err
	}

	image, info, err := loader.Acquire(key, path, thumbnail)
	imageRef := imageRef{
		image: image,
		info:  info,
		err:   err,
	}
	c.cache.SetWithTTL(key, imageRef, 0, 10*time.Minute)
	loader.Release(key)
	return image, info, err
}

func (c *ImageCache) Delete(path string) {
	c.cache.Del(path)
}

func newFileExistsCache() *ristretto.Cache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 24, // maximum cost of cache (16MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	metrics.AddRistretto("file_exists_cache", cache)
	return cache
}

type PathCache struct {
	cache *ristretto.Cache
}

func (c *PathCache) Get(id ImageId) (string, bool) {
	value, found := c.cache.Get((uint32)(id))
	if found {
		return value.(string), true
	}
	return "", false
}

func (c *PathCache) Set(id ImageId, path string) error {
	c.cache.Set((uint32)(id), path, (int64)(len(path)))
	return nil
}

func (c *PathCache) Delete(id ImageId) {
	c.cache.Del((uint32)(id))
}

func newPathCache() PathCache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10e3,    // number of keys to track frequency of (10k).
		MaxCost:     1 << 22, // maximum cost of cache (4MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	metrics.AddRistretto("path_cache", cache)
	return PathCache{
		cache: cache,
	}
}
