package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	img "photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/task"

	"github.com/karrick/godirwalk"
)

var errSkip = errors.New("skipping the rest")

func walkFiles(ctx context.Context, dir string, extensions []string, maxFiles int) <-chan string {
	out := make(chan string)
	go func() {
		finished := metrics.Elapsed(fmt.Sprintf("index %s", dir))
		defer finished()

		progress := newProgress("files", 0)
		defer progress.Done()

		files := 0
		err := godirwalk.Walk(dir, &godirwalk.Options{
			Unsorted: true,
			Callback: func(path string, _ *godirwalk.Dirent) error {
				if strings.Contains(path, "@eaDir") {
					return filepath.SkipDir
				}

				matched := false
				for _, ext := range extensions {
					if strings.HasSuffix(strings.ToLower(path), ext) {
						matched = true
						break
					}
				}
				if !matched {
					return nil
				}

				files++
				progress.Inc(1)
				select {
				case out <- path:
				case <-ctx.Done():
					return errSkip
				}
				if maxFiles > 0 && files >= maxFiles {
					return errSkip
				}
				return nil
			},
		})
		if err != nil && err != errSkip {
			log.Printf("index files error: %s\n", err.Error())
		}

		close(out)
	}()
	return out
}

// RunFiles scans directories for media files and updates the database.
// New files are added, missing files are removed, and the directory is
// marked as indexed.
func RunFiles(ctx context.Context, cfg Config, t *task.Task) error {
	if cfg.DB == nil {
		return nil
	}

	counter := t.Counter()
	defer close(counter)

	for _, dir := range t.Dirs {
		log.Printf("index files %s\n", dir)
		indexed := make(map[string]struct{})

		for path := range walkFiles(ctx, dir, cfg.Extensions, t.MaxPhotos) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			cfg.DB.Write(path, img.Info{}, img.AppendPath)
			indexed[path] = struct{}{}
			counter <- 1
		}

		<-cfg.DB.CommitBarrier()

		for ip := range cfg.DB.ListNonexistent(dir, indexed) {
			cfg.DB.Delete(ip.Id)
			if cfg.ThumbnailSink != nil {
				cfg.ThumbnailSink.Delete(uint32(ip.Id))
			}
		}

		cfg.DB.SetIndexed(dir)
		<-cfg.DB.CommitBarrier()
	}

	log.Println("index files done")
	return nil
}
