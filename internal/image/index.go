package image

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"photofield/internal/metrics"
	"strings"
	"time"

	"github.com/karrick/godirwalk"
)

var ErrSkip = errors.New("skipping the rest")

func walkFiles(dir string, extensions []string, maxFiles int) <-chan string {
	out := make(chan string)
	go func() {
		finished := metrics.Elapsed(fmt.Sprintf("index %s", dir))
		defer finished()

		lastLogTime := time.Now()
		files := 0
		err := godirwalk.Walk(dir, &godirwalk.Options{
			Unsorted: true,
			Callback: func(path string, walk_dir *godirwalk.Dirent) error {
				if strings.Contains(path, "@eaDir") {
					return filepath.SkipDir
				}

				suffix := ""
				for _, ext := range extensions {
					if strings.HasSuffix(strings.ToLower(path), ext) {
						suffix = ext
						break
					}
				}
				if suffix == "" {
					return nil
				}

				files++
				now := time.Now()
				if now.Sub(lastLogTime) > 1*time.Second {
					lastLogTime = now
					log.Printf("indexing %s %d files\n", dir, files)
				}
				out <- path
				if maxFiles > 0 && files >= maxFiles {
					return ErrSkip
				}
				return nil
			},
		})
		if err != nil && err != ErrSkip {
			log.Printf("Error indexing files: %s\n", err.Error())
		}

		close(out)
	}()
	return out
}
