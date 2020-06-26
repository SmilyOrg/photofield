package photofield

import (
	"log"
	"sync"
	"time"

	. "photofield/internal/storage"
)

type Collection struct {
	ListLimit int
	Dirs      []string
}

func (collection *Collection) GetPaths(source *ImageSource) <-chan string {
	log.Println("listing")
	preListing := time.Now()
	out := make(chan string)
	wg := &sync.WaitGroup{}
	wg.Add(len(collection.Dirs))
	for _, photoDir := range collection.Dirs {
		go source.ListImages(photoDir, collection.ListLimit, out, wg)
	}
	go func() {
		wg.Wait()
		listingElapsed := time.Since(preListing).Milliseconds()
		log.Printf("listing %4d ms all\n", listingElapsed)
		close(out)
	}()
	// photos := getPhotosFromPaths(paths)
	return out
}
