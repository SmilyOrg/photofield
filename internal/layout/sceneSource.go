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

	scenes        *ristretto.Cache
	scenesLoading sync.Map
}

type loadingScene struct {
	scene  *Scene
	loaded chan struct{}
}

type SceneConfig struct {
	Render     Render
	Collection Collection
	Layout     Layout
	Scene      Scene
}

func NewSceneSource() *SceneSource {
	var err error
	source := SceneSource{}
	source.scenes, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e4,     // number of keys to track frequency of, 10x max expected key count
		MaxCost:     1 << 27, // maximum size/cost of cache (128MiB)
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	AddRistrettoMetrics("scene_cache", source.scenes)
	return &source
}

func getCollectionKey(collection Collection) string {
	key := fmt.Sprintf("%v", collection.ListLimit)
	for _, dir := range collection.Dirs {
		key += " " + dir
	}
	return key
}

func getLayoutKey(layout Layout) string {
	key := fmt.Sprintf("%v %v %v", layout.SceneWidth, layout.ImageHeight, layout.Type)
	return key
}

func (source *SceneSource) getImageIds(collection Collection, imageSource *ImageSource) []ImageId {
	ids := make([]ImageId, 0)
	for id := range collection.GetIds(imageSource) {
		ids = append(ids, id)
	}
	return ids
}

func getSceneCost(scene *Scene) int64 {
	structCost := (int64)(unsafe.Sizeof(*scene))
	photosCost := (int64)(len(scene.Photos)) * (int64)(unsafe.Sizeof(scene.Photos[0]))
	solidsCost := (int64)(len(scene.Solids)) * (int64)(unsafe.Sizeof(scene.Solids[0]))
	textsCost := (int64)(len(scene.Texts)) * ((int64)(unsafe.Sizeof(scene.Solids[0])) + (int64)(100))
	return structCost + photosCost + solidsCost + textsCost
}

func (source *SceneSource) loadScene(config SceneConfig, imageSource *ImageSource) Scene {
	scene := source.DefaultScene
	ids := source.getImageIds(config.Collection, imageSource)
	scene.AddPhotosFromIdSlice(ids)

	layoutFinished := ElapsedWithCount("layout", len(ids))
	switch config.Layout.Type {
	case "timeline":
		LayoutTimeline(config.Layout, &scene, imageSource)

	case "album":
		LayoutAlbum(config.Layout, &scene, imageSource)

	case "square":
		LayoutSquare(&scene, imageSource)

	case "wall":
		LayoutWall(config.Layout, &scene, imageSource)

	default:
		LayoutAlbum(config.Layout, &scene, imageSource)
	}
	layoutFinished()

	if scene.RegionSource == nil {
		scene.RegionSource = &PhotoRegionSource{
			imageSource: imageSource,
		}
	}

	log.Printf("photos %d, scene %.0f x %.0f\n", len(scene.Photos), scene.Bounds.W, scene.Bounds.H)
	return scene
}

func (source *SceneSource) GetScene(config SceneConfig, imageSource *ImageSource, cacheKey string) *Scene {
	key := fmt.Sprintf("%v %v %v", getCollectionKey(config.Collection), getLayoutKey(config.Layout), cacheKey)

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

	scene := source.loadScene(config, imageSource)

	source.scenes.Set(key, &scene, getSceneCost(&scene))
	loading.scene = &scene
	close(loading.loaded)
	source.scenesLoading.Delete(key)
	return &scene
}
