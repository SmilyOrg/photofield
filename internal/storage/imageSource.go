package photofield

import (
	"errors"
	"fmt"
	"image"
	"os"
	"reflect"
	"sync"
	"time"
	"unsafe"

	decoder "photofield/internal/decoder"

	"github.com/dgraph-io/ristretto"
)

type ImageSource struct {
	imagesLoading sync.Map
	images        *ristretto.Cache
	infos         *ristretto.Cache
	// imageByPath       sync.Map
	// imageConfigByPath sync.Map
}

type imageRef struct {
	path  string
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
	source.images, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache
		// MaxCost:     1 << 27, // maximum cost of cache
		BufferItems: 64, // number of keys per Get buffer.
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			imageRef := value.(imageRef)
			// config := source.GetImageConfig(imageRef.path)
			// switch imageType := (*imageRef.image).(type) {
			// case image.YCbCr:
			// 	println("YCbCr")
			// default:
			// 	println("UNKNOWN")
			// }
			// return 1
			switch img := (*imageRef.image).(type) {

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
				panic(fmt.Sprintf("Unable to compute cost, unsupported image format %v", reflect.TypeOf(img).String()))
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
	source.infos, err = ristretto.NewCache(&ristretto.Config{
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

func (source *ImageSource) LoadImage(path string) (*image.Image, error) {
	// fmt.Printf("loading %s\n", path)
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	image, _, err := decoder.Decode(file)
	return &image, err
}

func (source *ImageSource) LoadImageInfo(path string) (*decoder.Info, error) {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	info, err := decoder.DecodeInfo(file)
	if err != nil {
		return nil, err
	}
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
			return value.(imageRef).image, nil
		} else {
			loading := &loadingImage{}
			// loadingImage.cond = sync.NewCond(&loadingImage.mutex)
			loading.mutex.Lock()
			stored, loaded := source.imagesLoading.LoadOrStore(path, loading)
			if loaded {
				// loadingImage.mutex.Unlock()
				// loadingImage = stored.(*LoadingImage)
				// loadingImage.mutex.Lock()
				// if loadingImage.cond != nil {
				// 	log.Printf("%v not found, try %v, waiting\n", path, try)
				// 	loadingImage.cond.Wait()
				// 	log.Printf("%v not found, try %v, waiting done\n", path, try)
				// 	imageRef := loadingImage.imageRef
				// 	loadingImage.mutex.Unlock()
				// 	return imageRef.image, nil
				// } else {
				// 	log.Printf("%v not found, try %v, done (no cond)\n", path, try)
				// 	return loadingImage.imageRef.image, nil
				// }
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
				return imageRef.image, nil
			} else {

				// log.Printf("%v not found, try %v, loading, mutex locked\n", path, try)
				image, err := source.LoadImage(path)
				if err != nil {
					panic(err)
				}

				imageRef := imageRef{
					path:  path,
					image: image,
				}
				source.images.Set(path, imageRef, 0)
				loading.imageRef = &imageRef
				// log.Printf("%v not found, try %v, loaded, broadcast\n", path, try)
				// cond := loadingImage.cond
				// loadingImage.cond = nil
				// cond.Broadcast()

				// source.imagesLoading.Delete(path)
				// log.Printf("%v not found, try %v, loaded, mutex unlocked\n", path, try)
				loading.mutex.Unlock()

				return image, nil
			}
		}
	}
	return nil, errors.New(fmt.Sprintf("Unable to get image after %v tries", tries))

	// imageRef := &ImageRef{}
	// imageRef.mutex.Lock()
	// stored, loaded := source.imageByPath.LoadOrStore(path, imageRef)

	// var loadedImage *image.Image

	// if loaded {
	// 	imageRef.mutex.Unlock()
	// 	imageRef = stored.(*ImageRef)
	// 	imageRef.mutex.RLock()
	// 	loadedImage = imageRef.image
	// 	imageRef.mutex.RUnlock()
	// } else {
	// 	image, err := source.LoadImage(path)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	imageRef.image = image
	// 	loadedImage = imageRef.image
	// 	imageRef.mutex.Unlock()
	// }

	// return loadedImage
}

func (source *ImageSource) GetImageInfo(path string) *decoder.Info {
	value, found := source.infos.Get(path)
	if found {
		return value.(*decoder.Info)
	} else {
		info, err := source.LoadImageInfo(path)
		if err != nil {
			panic(err)
		}
		source.infos.Set(path, info, 1)
		return info
	}
}
