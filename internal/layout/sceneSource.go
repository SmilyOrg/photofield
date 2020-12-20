package photofield

import (
	"fmt"
	"log"
	"sync"
	"unsafe"

	"github.com/dgraph-io/ristretto"

	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	. "photofield/internal/storage"
)

type SceneSource struct {
	DefaultScene Scene

	imageIds      *ristretto.Cache
	scenes        *ristretto.Cache
	scenesLoading sync.Map
}

type loadingScene struct {
	scene  *Scene
	loaded chan struct{}
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
		NumCounters: 1e4,     // number of keys to track frequency of, 10x max expected key count
		MaxCost:     1 << 27, // maximum size/cost of cache (128MiB)
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	source.scenes, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,     // number of keys to track frequency of, 10x max expected key count
		MaxCost:     1 << 27, // maximum size/cost of cache (128MiB)
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

	cost := (int64)(len(ids)) * (int64)(unsafe.Sizeof(ids[0]))

	source.imageIds.Set(key, ids, cost)
	return ids
}

func getSceneCost(scene *Scene) int64 {
	structCost := (int64)(unsafe.Sizeof(*scene))
	photosCost := (int64)(len(scene.Photos)) * (int64)(unsafe.Sizeof(scene.Photos[0]))
	solidsCost := (int64)(len(scene.Solids)) * (int64)(unsafe.Sizeof(scene.Solids[0]))
	textsCost := (int64)(len(scene.Texts)) * ((int64)(unsafe.Sizeof(scene.Solids[0])) + (int64)(100))
	return structCost + photosCost + solidsCost + textsCost
}

func (source *SceneSource) GetScene(config SceneConfig, imageSource *ImageSource, cacheKey string) *Scene {
	// fmt.Printf("scenes %3.0f%% hit ratio, cached %3d MiB, added %d MiB, evicted %d MiB, hits %d, misses %d\n",
	// 	source.scenes.Metrics.Ratio()*100,
	// 	(source.scenes.Metrics.CostAdded()-source.scenes.Metrics.CostEvicted())/1024/1024,
	// 	source.scenes.Metrics.CostAdded()/1024/1024,
	// 	source.scenes.Metrics.CostEvicted()/1024/1024,
	// 	source.scenes.Metrics.Hits(),
	// 	source.scenes.Metrics.Misses())

	key := fmt.Sprintf("%v %v %v", getCollectionKey(config.Collection), getLayoutKey(config.Layout), cacheKey)

	tries := 1000
	for try := 0; try < tries; try++ {
		value, found := source.scenes.Get(key)
		if found {
			return value.(*Scene)
		}

		loading := &loadingScene{}
		loading.loaded = make(chan struct{})

		stored, loaded := source.scenesLoading.LoadOrStore(key, loading)
		if loaded {
			loading = stored.(*loadingScene)
			<-loading.loaded
			return loading.scene
		}

		scene := source.DefaultScene
		ids := source.getImageIds(config.Collection, imageSource)
		scene.AddPhotosFromIdSlice(ids)

		layoutFinished := ElapsedWithCount("layout", len(ids))
		// LayoutTimelineEvents(config.Layout, &scene, imageSource)
		LayoutAlbum(config.Layout, &scene, imageSource)
		// LayoutSquare(&scene, imageSource)
		// LayoutWall(&config.Config, &scene, imageSource)
		layoutFinished()

		if scene.RegionSource == nil {
			scene.RegionSource = &PhotoRegionSource{
				imageSource: imageSource,
			}
		}

		log.Printf("photos %d, scene %.0f x %.0f\n", len(scene.Photos), scene.Bounds.W, scene.Bounds.H)

		source.scenes.Set(key, &scene, getSceneCost(&scene))
		loading.scene = &scene
		close(loading.loaded)
		source.scenesLoading.Delete(key)
	}

	panic(fmt.Sprintf("Unable to get scene after %v tries", tries))
}
