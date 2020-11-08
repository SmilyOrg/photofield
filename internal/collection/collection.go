package photofield

import (
	"sync"

	"github.com/gosimple/slug"

	. "photofield/internal"
	. "photofield/internal/storage"
)

type Collection struct {
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	ListLimit int      `json:"list_limit"`
	Dirs      []string `json:"dirs"`
}

func (collection *Collection) GenerateId() {
	collection.Id = slug.Make(collection.Name)
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
