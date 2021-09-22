package collection

import (
	"log"
	"os"
	"path/filepath"
	"photofield/internal/image"
	"sort"
	"time"

	"github.com/gosimple/slug"
)

type Collection struct {
	Id            string     `json:"id"`
	Name          string     `json:"name"`
	Layout        string     `json:"layout"`
	Limit         int        `json:"limit"`
	IndexLimit    int        `json:"index_limit"`
	ExpandSubdirs bool       `json:"expand_subdirs"`
	ExpandSort    string     `json:"expand_sort"`
	Dirs          []string   `json:"dirs"`
	IndexedAt     *time.Time `json:"indexed_at,omitempty"`
}

func (collection *Collection) GenerateId() {
	collection.Id = slug.Make(collection.Name)
}

func (collection *Collection) Expand() []Collection {
	collections := make([]Collection, 0)
	for _, photoDir := range collection.Dirs {
		dir, err := os.Open(photoDir)
		if err != nil {
			log.Fatalln("Unable to expand dir", photoDir)
		}
		defer dir.Close()

		list, _ := dir.Readdirnames(0)
		for _, name := range list {
			child := Collection{
				Name:       name,
				Dirs:       []string{filepath.Join(photoDir, name)},
				Limit:      collection.Limit,
				IndexLimit: collection.IndexLimit,
			}
			collections = append(collections, child)
		}
	}
	switch collection.ExpandSort {
	case "asc":
		sort.Slice(collections, func(i, j int) bool {
			return collections[i].Name < collections[j].Name
		})
	case "desc":
		sort.Slice(collections, func(i, j int) bool {
			return collections[i].Name > collections[j].Name
		})
	}
	return collections
}

func (collection *Collection) UpdateStatus(source *image.Source) {
	var earliestIndex *time.Time
	for _, dir := range collection.Dirs {
		info := source.GetDir(dir)
		if !info.DateTime.IsZero() && (earliestIndex == nil || info.DateTime.Before(*earliestIndex)) {
			earliestIndex = &info.DateTime
		}
	}
	collection.IndexedAt = earliestIndex
}

func (collection *Collection) GetInfos(source *image.Source, options image.ListOptions) <-chan image.SourcedInfo {
	return source.ListInfos(collection.Dirs, options)
}

func (collection *Collection) GetIds(source *image.Source) <-chan image.ImageId {
	out := make(chan image.ImageId)
	go func() {
		for path := range collection.GetPaths(source) {
			out <- source.GetImageId(path)
		}
		close(out)
	}()
	return out
}

func (collection *Collection) GetPaths(source *image.Source) <-chan string {
	limit := 0
	if collection.IndexLimit > 0 {
		limit = collection.IndexLimit
	}
	if collection.Limit > 0 {
		limit = collection.Limit
	}
	return source.ListImages(collection.Dirs, limit)
}
