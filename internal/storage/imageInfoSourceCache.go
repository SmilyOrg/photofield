package photofield

import (
	. "photofield/internal"

	"github.com/dgraph-io/ristretto"
)

type ImageInfoSourceCache struct {
	cache *ristretto.Cache
}

func NewImageInfoSourceCache() *ImageInfoSourceCache {
	var err error
	source := ImageInfoSourceCache{}
	source.cache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 27, // maximum cost of cache (128MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	return &source
}

func (source *ImageInfoSourceCache) Get(path string) (*ImageInfo, error) {
	value, found := source.cache.Get(path)
	if found {
		return value.(*ImageInfo), nil
	}
	return nil, nil
}

func (source *ImageInfoSourceCache) Set(path string, info *ImageInfo) error {
	source.cache.Set(path, info, 1)
	return nil
}
