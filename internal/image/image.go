package image

import (
	"fmt"
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

func (source *Source) decode(path string, reader io.ReadSeeker) (*image.Image, error) {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, "jpg") || strings.HasSuffix(lower, "jpeg") {
		image, err := codec.DecodeJpeg(reader)
		return &image, err
	}

	image, _, err := image.Decode(reader)
	return &image, err
}

func (source *Source) LoadImage(path string) (*image.Image, error) {
	// fmt.Printf("loading %s\n", path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return source.decode(path, file)
}

func (source *Source) GetImage(path string) (*image.Image, error) {
	tries := 1000
	for try := 0; try < tries; try++ {
		value, found := source.imageCache.Get(path)
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
				source.imageCache.Set(path, imageRef, 0)
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
	return nil, fmt.Errorf("unable to get image after %v tries", tries)
}

func (source *Source) GetSmallestThumbnail(path string) string {
	for i := range source.Images.Thumbnails {
		thumbnail := &source.Images.Thumbnails[i]
		thumbnailPath := thumbnail.GetPath(path)
		if source.Exists(thumbnailPath) {
			return thumbnailPath
		}
	}
	return ""
}

func (source *Source) LoadSmallestImage(path string) (*image.Image, error) {
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

func (source *Source) LoadImageColor(path string) (color.RGBA, error) {
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
