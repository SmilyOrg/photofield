package image

import (
	"photofield/internal/metrics"
	"unsafe"

	"github.com/dgraph-io/ristretto"
)

type InfoCache struct {
	cache *ristretto.Cache[uint32, Info]
}

func (c *InfoCache) Get(id ImageId) (Info, bool) {
	value, found := c.cache.Get((uint32)(id))
	if found {
		return value, true
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
	cache, err := ristretto.NewCache(&ristretto.Config[uint32, Info]{
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

func (c *InfoCache) Close() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Clear()
	c.cache.Close()
	c.cache = nil
}

type PathCache struct {
	cache *ristretto.Cache[uint32, string]
}

func (c *PathCache) Get(id ImageId) (string, bool) {
	value, found := c.cache.Get((uint32)(id))
	if found {
		return value, true
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
	cache, err := ristretto.NewCache(&ristretto.Config[uint32, string]{
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

func (c *PathCache) Close() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Clear()
	c.cache.Close()
	c.cache = nil
}
