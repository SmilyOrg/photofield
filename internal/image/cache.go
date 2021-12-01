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

func (c *InfoCache) Get(path string) (Info, bool) {
	value, found := c.cache.Get(path)
	if found {
		return value.(Info), true
	}
	return Info{}, false
}

func (c *InfoCache) Set(path string, info Info) error {
	c.cache.Set(path, info, (int64)(unsafe.Sizeof(info)))
	return nil
}

func (c *InfoCache) Delete(path string) {
	c.cache.Del(path)
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
	tries := 10
	key := path
	if thumbnail != nil {
		key += "|thumbnail|" + thumbnail.Name
	}
	for try := 0; try < tries; try++ {
		value, found := c.cache.Get(key)
		if found {
			imageRef := value.(imageRef)
			return imageRef.image, imageRef.info, imageRef.err
		} else {
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
	}
	return nil, Info{}, fmt.Errorf("unable to get image after %v tries", tries)
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
