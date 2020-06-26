package photofield

import (
	"fmt"
	"log"
	"time"

	"github.com/dgraph-io/ristretto"

	. "photofield/internal/collection"
	. "photofield/internal/display"
	. "photofield/internal/storage"
)

type SceneSource struct {
	DefaultScene Scene

	files  *ristretto.Cache
	scenes *ristretto.Cache
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
	source.files, err = ristretto.NewCache(&ristretto.Config{
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

func (source *SceneSource) GetScene(config SceneConfig, imageSource *ImageSource) *Scene {
	// fmt.Printf("%3.0f%% hit ratio, added %d MB, evicted %d MB, hits %d, misses %d\n",
	// 	source.scenes.Metrics.Ratio()*100,
	// 	source.scenes.Metrics.CostAdded()/1024/1024,
	// 	source.scenes.Metrics.CostEvicted()/1024/1024,
	// 	source.scenes.Metrics.Hits(),
	// 	source.scenes.Metrics.Misses())

	key := fmt.Sprintf("%v %v", config.Layout.SceneWidth, config.Layout.ImageHeight)

	value, found := source.scenes.Get(key)
	if found {
		return value.(*Scene)
	}

	scene := source.DefaultScene
	scene.AddPhotosFromPaths(config.Collection.GetPaths(imageSource))
	log.Printf("photos %d\n", len(scene.Photos))

	preLayout := time.Now()
	LayoutTimelineEvents(config.Layout, &scene, imageSource)
	layoutElapsed := time.Since(preLayout).Milliseconds()

	log.Printf("layout %4d ms all, %4.2f ms / photo\n", layoutElapsed, float64(layoutElapsed)/float64(len(scene.Photos)))
	log.Printf("scene %.0f %.0f\n", scene.Bounds.W, scene.Bounds.H)

	source.scenes.Set(key, &scene, 0)

	return &scene
}
