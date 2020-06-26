package photofield

import (
	"sync"

	. "photofield/internal"
	. "photofield/internal/storage"
)

type Collection struct {
	ListLimit int
	Dirs      []string
}

func (collection *Collection) GetIds(source *ImageSource) <-chan ImageId {
	out := make(chan ImageId)
	go func() {
		for path := range collection.GetPaths(source) {
			out <- source.GetImageId(path)
		}
		close(out)
	}()
	return out
}

func (collection *Collection) GetPaths(source *ImageSource) <-chan string {
	listingFinished := Elapsed("listing")
	out := make(chan string)
	wg := &sync.WaitGroup{}
	wg.Add(len(collection.Dirs))
	for _, photoDir := range collection.Dirs {
		go source.ListImages(photoDir, collection.ListLimit, out, wg)
	}
	go func() {
		wg.Wait()
		listingFinished()
		close(out)
	}()
	return out
}
