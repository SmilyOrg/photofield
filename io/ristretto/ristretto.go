package ristretto

import (
	"context"
	"fmt"
	"image"
	"log"
	"photofield/internal/metrics"
	"photofield/io"
	"reflect"
	"time"
	"unsafe"

	_ "image/jpeg"
	_ "image/png"

	drist "github.com/dgraph-io/ristretto"
	"github.com/dgraph-io/ristretto/z"
)

type Ristretto struct {
	cache *drist.Cache[IdWithName, io.Result]
}

type IdWithSize struct {
	Id   io.ImageId
	Size io.Size
}

type IdWithName struct {
	Id   io.ImageId
	Name string
}

func (ids IdWithSize) String() string {
	return fmt.Sprintf("%6d %4d %4d", ids.Id, ids.Size.X, ids.Size.Y)
}

func New() *Ristretto {
	maxSizeBytes := int64(256000000)

	cache, err := drist.NewCache(&drist.Config[IdWithName, io.Result]{
		NumCounters: 1e6,          // number of keys to track frequency of
		MaxCost:     maxSizeBytes, // maximum cost of cache
		BufferItems: 64,           // number of keys per Get buffer
		Metrics:     true,
		Cost:        cost,
		KeyToHash:   keyToHash,
	})
	if err != nil {
		panic(err)
	}
	metrics.AddRistretto("image_cache", cache)
	return &Ristretto{
		cache: cache,
	}
}

func (r *Ristretto) Close() error {
	if r == nil || r.cache == nil {
		return nil
	}
	r.cache.Clear()
	r.cache.Close()
	r.cache = nil
	return nil
}

func keyToHash(k IdWithName) (uint64, uint64) {
	str := fmt.Sprintf("%d %s", k.Id, k.Name)
	return z.KeyToHash(str)
}

func (r *Ristretto) Name() string {
	return "ristretto"
}

func (r *Ristretto) Size(size io.Size) io.Size {
	return io.Size{}
}

func (r *Ristretto) GetDurationEstimate(size io.Size) time.Duration {
	return 80 * time.Nanosecond
}

func (r *Ristretto) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	idn := IdWithName{Id: id}
	value, found := r.cache.Get(idn)
	if found {
		return value
	}
	return io.Result{}
}

func (r *Ristretto) GetWithName(ctx context.Context, id io.ImageId, name string) io.Result {
	idn := IdWithName{
		Id:   id,
		Name: name,
	}
	value, found := r.cache.Get(idn)
	if found {
		return value
	}
	return io.Result{}
}

func (r *Ristretto) SetWithName(ctx context.Context, id io.ImageId, name string, v io.Result) bool {
	idn := IdWithName{
		Id:   id,
		Name: name,
	}
	return r.cache.SetWithTTL(idn, v, 0, 10*time.Minute)
}

func (r *Ristretto) Set(ctx context.Context, id io.ImageId, path string, v io.Result) bool {
	idn := IdWithName{Id: id}
	return r.cache.SetWithTTL(idn, v, 0, 10*time.Minute)
}

func cost(r io.Result) int64 {
	img := r.Image
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

	case *image.Paletted:
		return int64(unsafe.Sizeof(*img)) +
			int64(cap(img.Pix))*int64(unsafe.Sizeof(img.Pix[0])) +
			int64(cap(img.Palette))*int64(unsafe.Sizeof(img.Pix[0]))

	case nil:
		return 1

	default:
		log.Printf("Unable to compute cost, unsupported image format %v", reflect.TypeOf(img))
		// Fallback image size (10MB)
		return 10000000
	}
}
