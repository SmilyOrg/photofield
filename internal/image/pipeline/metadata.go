package pipeline

import (
	"context"
	"log"
	"sync"

	img "photofield/internal/image"
	"photofield/internal/tag"
)

// MetadataExtractor extracts metadata from files
type MetadataExtractor interface {
	DecodeInfo(path string, info *img.Info) ([]tag.Tag, error)
}

// processMetadata extracts metadata from files and writes to DB
func processMetadata(ctx context.Context, db *img.Database, decoder MetadataExtractor,
	in <-chan fileRef, workers int, enableTags bool, counter chan<- int) <-chan fileWithMeta {
	out := make(chan fileWithMeta, 100)

	var wg sync.WaitGroup
	progress := newProgress("metadata", 0)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for file := range in {
				info := img.Info{}
				tags, err := decoder.DecodeInfo(file.Path, &info)

				if err != nil {
					log.Printf("index error: metadata extract %s: %v\n", file.Path, err)
					continue
				}

				// Write to database immediately
				db.Write(file.Path, info, img.UpdateMeta)
				if enableTags && len(tags) > 0 {
					db.WriteTags(file.ID, tags)
				}

				// Update progress
				progress.Inc(1)

				// Update task progress
				if counter != nil {
					select {
					case counter <- 1:
					case <-ctx.Done():
						return
					}
				}

				select {
				case out <- fileWithMeta{
					fileRef: file,
					Info:    info,
					Tags:    tags,
				}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		progress.Done()
		close(out)
	}()

	return out
}

// metadataToFileRef converts fileWithMeta back to fileRef for stages that don't need metadata
func metadataToFileRef(in <-chan fileWithMeta) <-chan fileRef {
	out := make(chan fileRef, 100)
	go func() {
		defer close(out)
		for fm := range in {
			out <- fm.fileRef
		}
	}()
	return out
}
