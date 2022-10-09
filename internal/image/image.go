package image

import (
	"image"
	"image/color"
	"io"
	"os"
	"photofield/internal/codec"
	"strings"
	"time"

	"github.com/EdlinOrg/prominentcolor"
)

type Size = image.Point

type loadingImage struct {
	imageRef imageRef
	loaded   chan struct{}
}

type imageRef struct {
	err   error
	info  Info
	image image.Image
}

func (source *Source) Exists(path string) bool {
	value, found := source.fileExistsCache.Get(path)
	if found {
		return value.(bool)
	}
	_, err := os.Stat(path)

	exists := !os.IsNotExist(err)
	source.fileExistsCache.SetWithTTL(path, exists, 1, 1*time.Minute)
	return exists
}

func (source *Source) decode(path string, reader io.ReadSeeker) (image.Image, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, "jpg") || strings.HasSuffix(lower, "jpeg") {
		image, err := codec.DecodeJpeg(reader)
		return image, err
	}

	image, _, err := image.Decode(reader)
	return image, err
}

func (source *Source) LoadImage(path string) (image.Image, error) {
	// fmt.Printf("loading %s\n", path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return source.decode(path, file)
}

func (source *Source) Acquire(key string, path string, thumbnail *Thumbnail) (image.Image, Info, error) {
	// log.Printf("%v acquire, %v\n", key, source.imagesLoadingCount)
	source.imagesLoadingCount++
	loading := &loadingImage{}
	loading.loaded = make(chan struct{})
	stored, loaded := source.imagesLoading.LoadOrStore(key, loading)
	if loaded {
		loading = stored.(*loadingImage)
		// log.Printf("%v blocking on channel\n", key)
		<-loading.loaded
		// log.Printf("%v channel unblocked\n", key)
		imageRef := loading.imageRef
		return imageRef.image, Info{}, imageRef.err
	} else {
		// log.Printf("%v not found, loading, mutex locked\n", key)
		var image image.Image
		info := Info{}
		var err error
		if thumbnail == nil {
			// log.Printf("%v loading\n", key)
			image, err = source.LoadImage(path)
		} else {
			thumbnailPath := thumbnail.GetPath(path)
			if thumbnailPath != "" {
				// log.Printf("%v loading thumbnail path %v\n", key, thumbnailPath)
				image, err = source.LoadImage(thumbnailPath)
			} else {
				// log.Printf("%v loading embedded %v\n", key, thumbnail.Exif)
				image, info, err = source.decoder.DecodeImage(path, thumbnail.Exif)
			}
		}
		imageRef := imageRef{
			image: image,
			err:   err,
		}
		loading.imageRef = imageRef
		// log.Printf("%v loaded, closing channel\n", key)
		close(loading.loaded)
		return image, info, err
	}
}

func (source *Source) Release(key string) {
	source.imagesLoading.Delete(key)
	source.imagesLoadingCount--
	// log.Printf("%v loaded, map entry deleted\n", key)
	// log.Printf("%v release, %v\n", key, source.imagesLoadingCount)
}

func (source *Source) GetImage(path string) (image.Image, Info, error) {
	return source.imageCache.GetOrLoad(path, nil, source)
}

func (source *Source) GetImageOrThumbnail(path string, thumbnail *Thumbnail) (image.Image, Info, error) {
	return source.imageCache.GetOrLoad(path, thumbnail, source)
}

func (source *Source) OpenSmallestThumbnail(path string, minSize int) (*os.File, error) {
	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
		if thumbnail.Width < minSize || thumbnail.Height < minSize {
			continue
		}
		thumbnailPath := thumbnail.GetPath(path)
		file, err := os.Open(thumbnailPath)
		if err != nil {
			continue
		}
		return file, nil
	}
	return os.Open(path)
}

func (source *Source) LoadSmallestImage(path string) (image.Image, error) {
	for i := range source.Thumbnails {
		thumbnail := &source.Thumbnails[i]
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

func (source *Source) LoadImageColor(path string) (color.RGBA, error) {
	colorImage, err := source.LoadSmallestImage(path)
	if err != nil {
		return color.RGBA{}, err
	}
	centroids, err := prominentcolor.KmeansWithAll(1, colorImage, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, prominentcolor.GetDefaultMasks())
	if err != nil {
		centroids, err = prominentcolor.KmeansWithAll(1, colorImage, prominentcolor.ArgumentDefault, prominentcolor.DefaultSize, make([]prominentcolor.ColorBackgroundMask, 0))
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
