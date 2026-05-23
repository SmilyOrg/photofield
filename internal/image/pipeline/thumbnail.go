package pipeline

import (
	"bytes"
	"context"
	goio "io"
	"log"
	"sync"

	pio "photofield/internal/io"
)

// ThumbnailSource can load or generate thumbnails
type ThumbnailSource interface {
	// Try to load existing thumbnail
	Reader(ctx context.Context, id pio.ImageId, path string, callback func(goio.ReadSeeker, error))
}

// ThumbnailGenerator can generate thumbnails from originals
type ThumbnailGenerator interface {
	Get(ctx context.Context, id pio.ImageId, path string) pio.Result
}

// ThumbnailSink can save and delete generated thumbnails
type ThumbnailSink interface {
	SetWithBuffer(ctx context.Context, id pio.ImageId, path string, buf *bytes.Buffer, r pio.Result) bool
	Delete(id uint32) error
}

// processThumbnails loads or generates thumbnails and calls process synchronously
// for each file while the thumbnail reader is still valid.
// For cached thumbnails the reader is owned by the sqlite statement and is only
// valid inside the Reader callback; process is therefore called from within that
// callback so no copy is needed.
// For generated thumbnails the reader is backed by a bytes.Buffer that outlives
// the call; process is called after generation.
func processThumbnails(ctx context.Context,
	sources []ThumbnailSource,
	generators []ThumbnailGenerator,
	sink ThumbnailSink,
	in <-chan fileWithMeta,
	workers int,
	counter chan<- int,
	process func(context.Context, fileWithThumb),
) {
	var wg sync.WaitGroup
	progress := newMultiProgress("thumbnails", 0, "generated")

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for file := range in {
				id := pio.ImageId(file.ID)

				// Try loading existing thumbnail from cache.
				// Call process synchronously inside the Reader callback so the
				// sqlite-backed reader is still valid when process reads from it.
				loaded := false
				for _, src := range sources {
					src.Reader(ctx, id, file.Path, func(rs goio.ReadSeeker, err error) {
						if err != nil || rs == nil {
							return
						}
						loaded = true
						process(ctx, fileWithThumb{
							fileRef:     file.fileRef,
							Thumb:       rs,
							Orientation: pio.Orientation(file.Info.Orientation),
							Missing:     file.Missing,
						})
					})
					if loaded {
						break
					}
				}

				if !loaded {
					// Generate thumbnail.
					var r pio.Result
					for _, gen := range generators {
						if gens, ok := gen.(interface {
							GetWithSize(context.Context, pio.ImageId, string, pio.Size) pio.Result
						}); ok {
							r = gens.GetWithSize(ctx, id, file.Path, pio.Size(file.Info.Size()))
						}
						if r.Image == nil || r.Error != nil {
							r = gen.Get(ctx, id, file.Path)
						}
						if r.Image != nil && r.Error == nil {
							break
						}
					}

					if r.Error != nil {
						log.Printf("index error: thumbnail generate %s: %v\n", file.Path, r.Error)
						continue
					}

					if r.Orientation == pio.SourceInfoOrientation {
						if !file.Info.Orientation.IsZero() {
							r.Orientation = pio.Orientation(file.Info.Orientation)
						}
					}

					var buf bytes.Buffer
					if !sink.SetWithBuffer(ctx, id, file.Path, &buf, r) {
						log.Printf("index error: thumbnail save %s: failed\n", file.Path)
						continue
					}

					// bytes.NewReader is backed by the buf above — always safe to use
					// outside the callback.
					progress.IncCounter("generated", 1)
					process(ctx, fileWithThumb{
						fileRef:     file.fileRef,
						Thumb:       bytes.NewReader(buf.Bytes()),
						Orientation: pio.Orientation(file.Info.Orientation),
						Missing:     file.Missing,
					})
				}

				progress.Inc(1)

				if counter != nil {
					select {
					case counter <- 1:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	progress.Done()
}
