package pipeline

import (
	"context"
	"log"

	img "photofield/internal/image"
)

// fileSource produces fileRef items from database based on force flag and missing criteria.
// If force is true, all files in dirs are sourced; otherwise only files matching missing are sourced.
func fileSource(ctx context.Context, db *img.Database, dirs []string, maxPhotos int, force bool, missing img.Missing) <-chan fileRef {
	out := make(chan fileRef, 100)

	go func() {
		defer close(out)

		var candidates <-chan img.MissingInfo

		if force {
			log.Println("index source all files (forced)")
			all := db.ListIdPaths(dirs, maxPhotos)
			c := make(chan img.MissingInfo, 100)
			go func() {
				defer close(c)
				for ip := range all {
					c <- img.MissingInfo{
						Id:      ip.Id,
						Path:    ip.Path,
						Missing: missing,
					}
				}
			}()
			candidates = c
		} else {
			log.Printf("index source files missing %+v\n", missing)
			candidates = db.ListMissing(dirs, maxPhotos, missing)
		}

		// Stream candidates
		count := 0
		for candidate := range candidates {
			select {
			case out <- fileRef{ID: candidate.Id, Path: candidate.Path}:
				count++
			case <-ctx.Done():
				log.Printf("index error: file source cancelled after %d files\n", count)
				return
			}
		}

		log.Printf("index sourced %d files\n", count)
	}()

	return out
}

// fileSourceWithMetadata produces fileWithMeta items from database with metadata already loaded.
// If force is true, all files are sourced; otherwise only files missing requested contents are sourced.
func fileSourceWithMetadata(ctx context.Context, db *img.Database, dirs []string, maxPhotos int, force bool, includeEmbedding bool) <-chan fileWithMeta {
	out := make(chan fileWithMeta, 100)
	started := make(chan struct{})

	go func() {
		defer close(out)

		// First get file references that need work
		var candidates <-chan img.MissingInfo

		if force {
			// Force contents: select all files
			log.Println("index source all files (force contents)")
			all := db.ListIdPaths(dirs, maxPhotos)
			c := make(chan img.MissingInfo, 100)
			go func() {
				defer close(c)
				for ip := range all {
					c <- img.MissingInfo{
						Id:   ip.Id,
						Path: ip.Path,
						Missing: img.Missing{
							Color:     true,
							Embedding: includeEmbedding,
						},
					}
				}
			}()
			candidates = c
		} else {
			// Query for missing contents
			missing := img.Missing{Color: true, Embedding: includeEmbedding}
			log.Printf("index source files missing %+v\n", missing)
			candidates = db.ListMissing(dirs, maxPhotos, missing)
		}

		// Signal we're ready to start processing
		close(started)

		// Batch load metadata for candidates
		// Collect IDs in batches to avoid excessive memory use
		batchSize := 100
		ids := make([]img.ImageId, 0, batchSize)
		idToPath := make(map[img.ImageId]string)
		idToMissing := make(map[img.ImageId]img.Missing)

		candidateCount := 0
		for candidate := range candidates {
			candidateCount++
			ids = append(ids, candidate.Id)
			idToPath[candidate.Id] = candidate.Path
			idToMissing[candidate.Id] = candidate.Missing

			// Process batch when full
			if len(ids) >= batchSize {
				log.Printf("index source batch %d files\n", len(ids))
				results := db.GetBatch(ids)
				for result := range results {
					missingInfo := idToMissing[result.Id]
					select {
					case out <- fileWithMeta{
						fileRef: fileRef{ID: result.Id, Path: idToPath[result.Id]},
						Info:    result.Info,
						Tags:    nil,
						Missing: missingInfo,
					}:
					case <-ctx.Done():
						return
					}
				}
				for _, id := range ids {
					delete(idToPath, id)
					delete(idToMissing, id)
				}
				ids = ids[:0]
			}
		}

		log.Printf("index source received %d candidates total\n", candidateCount)

		// Process remaining batch
		if len(ids) > 0 {
			log.Printf("index source batch %d files (last)\n", len(ids))
			results := db.GetBatch(ids)
			for result := range results {
				missingInfo := idToMissing[result.Id]
				select {
				case out <- fileWithMeta{
					fileRef: fileRef{ID: result.Id, Path: idToPath[result.Id]},
					Info:    result.Info,
					Tags:    nil,
					Missing: missingInfo,
				}:
				case <-ctx.Done():
					return
				}
			}
			for _, id := range ids {
				delete(idToPath, id)
				delete(idToMissing, id)
			}
		}
	}()

	<-started // Wait for goroutine to start before returning channel
	return out
}

// fork duplicates a fileRef channel into two separate channels
func fork(in <-chan fileRef) (<-chan fileRef, <-chan fileRef) {
	out1 := make(chan fileRef, 100)
	out2 := make(chan fileRef, 100)

	go func() {
		defer close(out1)
		defer close(out2)

		for item := range in {
			out1 <- item
			out2 <- item
		}
	}()

	return out1, out2
}
