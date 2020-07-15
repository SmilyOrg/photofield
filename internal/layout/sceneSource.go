package photofield

import (
	"fmt"
	"log"

	"github.com/dgraph-io/ristretto"

	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	. "photofield/internal/storage"
)

type SceneSource struct {
	DefaultScene Scene

	imageIds *ristretto.Cache
	scenes   *ristretto.Cache
}

type SceneConfig struct {
	Config     RenderConfig
	Collection Collection
	Layout     LayoutConfig
	Scene      Scene
}

func NewSceneSource() *SceneSource {
	var err error
	source := SceneSource{}
	source.imageIds, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 27, // maximum cost of cache (128MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	source.scenes, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 27, // maximum cost of cache (128MB).
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	return &source
}

func getCollectionKey(collection Collection) string {
	key := fmt.Sprintf("%v", collection.ListLimit)
	for _, dir := range collection.Dirs {
		key += " " + dir
	}
	return key
}

func getLayoutKey(layout LayoutConfig) string {
	key := fmt.Sprintf("%v %v", layout.SceneWidth, layout.ImageHeight)
	return key
}

func (source *SceneSource) getImageIds(collection Collection, imageSource *ImageSource) []ImageId {
	key := getCollectionKey(collection)

	value, found := source.imageIds.Get(key)
	if found {
		return value.([]ImageId)
	}

	ids := make([]ImageId, 0)
	for id := range collection.GetIds(imageSource) {
		ids = append(ids, id)
	}

	source.imageIds.Set(key, ids, 1)
	return ids
}

func (source *SceneSource) GetScene(config SceneConfig, imageSource *ImageSource) *Scene {
	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
	// 	source.scenes.Metrics.Ratio()*100,
	// 	source.scenes.Metrics.CostAdded()/1024/1024,
	// 	source.scenes.Metrics.CostEvicted()/1024/1024,
	// 	source.scenes.Metrics.Hits(),
	// 	source.scenes.Metrics.Misses())

	key := fmt.Sprintf("%v %v", getCollectionKey(config.Collection), getLayoutKey(config.Layout))

	value, found := source.scenes.Get(key)
	if found {
		return value.(*Scene)
	}

	scene := source.DefaultScene
	ids := source.getImageIds(config.Collection, imageSource)
	scene.AddPhotosFromIdSlice(ids)

	layoutFinished := ElapsedWithCount("layout", len(ids))
	LayoutTimelineEvents(config.Layout, &scene, imageSource)
	// LayoutSquare(&scene, imageSource)
	// LayoutWall(&config.Config, &scene, imageSource)
	layoutFinished()

	if scene.RegionSource == nil {
		scene.RegionSource = &PhotoRegionSource{
			imageSource: imageSource,
		}
	}

	log.Printf("photos %d, scene %.0f x %.0f\n", len(scene.Photos), scene.Bounds.W, scene.Bounds.H)

	source.scenes.Set(key, &scene, 1)

	return &scene
}
