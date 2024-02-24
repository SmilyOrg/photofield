package collection

import (
	"log"
	"os"
	"path/filepath"
	"photofield/internal/clip"
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
	IndexedCount  int        `json:"indexed_count"`
	InvalidatedAt *time.Time `json:"-"`
}

func (collection *Collection) GenerateId() {
	collection.Id = slug.Make(collection.Name)
}

func (collection *Collection) UpdatedAt() time.Time {
	if collection.InvalidatedAt != nil && collection.IndexedAt != nil {
		if collection.InvalidatedAt.After(*collection.IndexedAt) {
			return *collection.InvalidatedAt
		}
		return *collection.IndexedAt
	} else if collection.InvalidatedAt != nil {
		return *collection.InvalidatedAt
	} else if collection.IndexedAt != nil {
		return *collection.IndexedAt
	}
	return time.Time{}
}

func (collection *Collection) Expand() []Collection {
	collections := make([]Collection, 0)
	for _, collectionDir := range collection.Dirs {
		dir, err := os.Open(collectionDir)
		if err != nil {
			log.Fatalln("Unable to expand dir", collectionDir)
		}
		defer dir.Close()

		list, _ := dir.ReadDir(0)
		for _, entry := range list {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			child := Collection{
				Name:       name,
				Dirs:       []string{filepath.Join(collectionDir, name)},
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

func (collection *Collection) UpdateIndexedAt(source *image.Source) {
	var earliestIndex *time.Time
	for _, dir := range collection.Dirs {
		info := source.GetDir(dir)
		if !info.DateTime.IsZero() && (earliestIndex == nil || info.DateTime.Before(*earliestIndex)) {
			earliestIndex = &info.DateTime
		}
	}
	collection.IndexedAt = earliestIndex
}

func (collection *Collection) UpdateIndexedCount(source *image.Source) {
	collection.IndexedCount = source.GetDirsCount(collection.Dirs)
}

func (collection *Collection) GetInfos(source *image.Source, options image.ListOptions) (<-chan image.SourcedInfo, image.Dependencies) {
	return source.ListInfos(collection.Dirs, options)
}

func (collection *Collection) GetSimilar(source *image.Source, embedding clip.Embedding, options image.ListOptions) <-chan image.SimilarityInfo {
	return source.ListSimilar(collection.Dirs, embedding, options)
}

func (collection *Collection) GetIds(source *image.Source) <-chan image.ImageId {
	limit := 0
	if collection.IndexLimit > 0 {
		limit = collection.IndexLimit
	}
	if collection.Limit > 0 {
		limit = collection.Limit
	}
	return source.ListImageIds(collection.Dirs, limit)
}

func (collection *Collection) GetIdsUint32(source *image.Source) <-chan uint32 {
	return image.IdsToUint32(collection.GetIds(source))
}
