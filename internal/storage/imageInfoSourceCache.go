package photofield

import (
	. "photofield/internal"
	"unsafe"

	"github.com/dgraph-io/ristretto"
)

type ImageInfoSourceCache struct {
	Cache *ristretto.Cache
}

func NewImageInfoSourceCache() *ImageInfoSourceCache {
	var err error
	source := ImageInfoSourceCache{}
	source.Cache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,     // number of keys to track frequency of (1M).
		MaxCost:     1 << 24, // maximum cost of cache (16MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	return &source
}

func (source *ImageInfoSourceCache) Get(path string) (ImageInfo, bool) {
	value, found := source.Cache.Get(path)
	if found {
		return value.(ImageInfo), true
	}
	return ImageInfo{}, false
}

func (source *ImageInfoSourceCache) Set(path string, info ImageInfo) error {
	source.Cache.Set(path, info, (int64)(unsafe.Sizeof(info)))
	return nil
}
