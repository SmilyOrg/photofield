package pipeline

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"photofield/internal/ai"
	img "photofield/internal/image"
)

// FaceDetector interface for face detection
type FaceDetector interface {
	DetectFaces(r io.Reader) ([]ai.Face, error)
}

// isVideo checks if a file extension is in the video extensions list
func isVideo(ext string, videoExtensions []string) bool {
	for _, e := range videoExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// processFaces detects faces from original files
// Takes fileWithContents as input to ensure contents are extracted first
func processFaces(ctx context.Context,
	db *img.Database,
	detector FaceDetector,
	in <-chan fileWithContents,
	workers int,
	maxFileSize int64,
	videoExtensions []string,
	counter chan<- int) {

	var wg sync.WaitGroup
	progress := newMultiProgress("faces", 0, "detected", "skipped")

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for file := range in {
				// Faces are always needed if this file comes through
				// (FileSource already filtered to files needing faces)

				// Open original file
				f, err := os.Open(file.Path)
				if err != nil {
					log.Printf("index error: open for faces %s: %v\n", file.Path, err)
					continue
				}

				// Check file size
				stat, err := f.Stat()
				if err != nil {
					f.Close()
					log.Printf("index error: stat for faces %s: %v\n", file.Path, err)
					continue
				}

				if stat.Size() > maxFileSize {
					f.Close()
					progress.IncCounter("skipped", 1)
					progress.Inc(1)
					continue
				}

				// Skip video files - face detection is only supported for images
				if isVideo(filepath.Ext(file.Path), videoExtensions) {
					f.Close()
					progress.IncCounter("skipped", 1)
					progress.Inc(1)
					continue
				}

				// Detect faces
				faces, err := detector.DetectFaces(f)
				f.Close()

				if err != nil && err != ai.ErrNotAvailable {
					log.Printf("index error: detect faces %s: %v\n", file.Path, err)
					continue
				}

				// Write to database (always, to mark as processed even if no faces found)
				db.WriteFaces(file.ID, faces)
				if len(faces) > 0 {
					progress.IncCounter("detected", len(faces))
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
			}
		}()
	}

	wg.Wait()
	progress.Done()
}
