package scene

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/dgraph-io/ristretto"
	gonanoid "github.com/matoous/go-nanoid/v2"

	"photofield/internal/collection"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/layout/shuffle"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"photofield/internal/search"
)

type SceneSource struct {
	DefaultScene render.Scene

	maxSize    int64
	sceneCache *ristretto.Cache[string, *render.Scene]
	scenes     sync.Map
}

type loadingScene struct {
	scene  *render.Scene
	loaded chan struct{}
}

type storedScene struct {
	scene  *render.Scene
	config SceneConfig
}

type SceneConfig struct {
	Render     render.Render
	Collection *collection.Collection
	Layout     layout.Layout
	Scene      render.Scene
}

func NewSceneSource() *SceneSource {
	var err error
	source := SceneSource{
		maxSize: 1 << 26, // 67 MB
	}
	source.sceneCache, err = ristretto.NewCache(&ristretto.Config[string, *render.Scene]{
		NumCounters: 10000,   // number of keys to track frequency of, 10x max expected key count
		MaxCost:     1 << 50, // maximum size/cost of cache, managed externally
		BufferItems: 64,      // number of keys per Get buffer.
		Metrics:     true,
	})
	if err != nil {
		panic(err)
	}
	metrics.AddRistretto("scene_cache", source.sceneCache)
	return &source
}

func getSceneCost(scene *render.Scene) int64 {
	structCost := (int64)(unsafe.Sizeof(*scene))
	photosCost := (int64)(len(scene.Photos)) * (int64)(unsafe.Sizeof(scene.Photos[0]))
	solidsCost := (int64)(len(scene.Solids)) * (int64)(unsafe.Sizeof(scene.Solids[0]))
	textsCost := (int64)(len(scene.Texts)) * ((int64)(unsafe.Sizeof(scene.Solids[0])) + (int64)(100))
	return structCost + photosCost + solidsCost + textsCost
}

func (source *SceneSource) loadScene(config SceneConfig, imageSource *image.Source) *render.Scene {

	log.Printf("scene loading %v", config.Collection.Id)

	scene := source.DefaultScene
	scene.CreatedAt = time.Now()
	scene.Loading = true
	scene.Search = config.Scene.Search

	// Compute shuffle seed for SQL ordering (UnixMilli is important for LCG random shuffling)
	shuffleSeed := shuffle.TruncateTime(shuffle.Order(config.Layout.Order), scene.CreatedAt).UnixMilli()

	// Add shuffle dependency if order is a shuffle type
	switch config.Layout.Order {
	case layout.ShuffleHourly, layout.ShuffleDaily, layout.ShuffleWeekly, layout.ShuffleMonthly:
		scene.Dependencies = append(scene.Dependencies, &render.ShuffleDependency{
			Order: shuffle.Order(config.Layout.Order),
		})
	}

	go func() {
		finished := metrics.Elapsed("scene load " + config.Collection.Id)

		var expression search.Expression
		if scene.Search != "" {
			searchDone := metrics.Elapsed("search")
			q, err := search.Parse(scene.Search)
			if err != nil && scene.Error == "" {
				scene.Error = fmt.Sprintf("parse failed: %s", err.Error())
			}

			scene.SearchTokens = q.Tokens()
			expression, err = q.Expression()
			if err != nil && scene.Error == "" {
				scene.Error = err.Error()
			}

			// If an image is specified, get its embedding
			if expression.Image.Present {
				embedding, err := imageSource.GetImageEmbedding(image.ImageId(expression.Image.Value))
				if err != nil {
					scene.Error = fmt.Sprintf("image embed failed: %s", err.Error())
				}
				scene.SearchEmbedding = embedding
			}

			// If no embedding yet, embed the text
			if scene.SearchEmbedding == nil && scene.Error == "" && expression.Text != "" {
				text := expression.Text
				done := metrics.Elapsed("search embed")
				embedding, err := imageSource.Clip.EmbedText(text)
				done()
				if err != nil {
					log.Println("search embed failed")
					scene.Error = fmt.Sprintf("text embed failed: %s", err.Error())
				}
				scene.SearchEmbedding = embedding
			}

			searchDone()
		}

		orderBySimilarity :=
			scene.SearchEmbedding != nil &&
				!expression.HasQualifiers([]string{"img"}) &&
				(config.Layout.Type != layout.Map)

		// Default threshold for non-similarity-order search
		if scene.SearchEmbedding != nil && !orderBySimilarity && !expression.Threshold.Present {
			expression.Threshold.Present = true
			expression.Threshold.Value = 0.262
		}

		if config.Layout.Type == layout.Highlights {
			infos := imageSource.ListInfosEmb(config.Collection.Dirs, image.ListOptions{
				OrderBy:     image.ListOrder(config.Layout.Order),
				ShuffleSeed: shuffleSeed,
				Limit:       config.Collection.Limit,
			})
			layout.LayoutHighlights(infos, config.Layout, &scene, imageSource)

		} else if orderBySimilarity {
			infos := config.Collection.GetSimilar(imageSource, scene.SearchEmbedding, image.ListOptions{
				Limit: config.Collection.Limit,
			})

			switch config.Layout.Type {
			case layout.Strip:
				sinfos := image.SimilarityInfosToSourcedInfos(infos)
				layout.LayoutStrip(sinfos, config.Layout, &scene, imageSource)
			default:
				layout.LayoutSearch(infos, config.Layout, &scene, imageSource)
			}
		} else {
			var infos <-chan image.SourcedInfo
			if expression.Filter.Value == "knn" {
				infos = imageSource.ListKnn(config.Collection.Dirs, image.ListOptions{
					OrderBy:     image.ListOrder(config.Layout.Order),
					ShuffleSeed: shuffleSeed,
					Limit:       config.Collection.Limit,
					Expression:  expression,
				})
			} else {
				// Normal order
				var deps image.Dependencies
				// Normal order
				var extensions []string
				if strings.Contains(config.Layout.Tweaks, "imageonly") {
					extensions = imageSource.Images.Extensions
				}
				infos, deps = config.Collection.GetInfos(imageSource, image.ListOptions{
					OrderBy:     image.ListOrder(config.Layout.Order),
					ShuffleSeed: shuffleSeed,
					Limit:       config.Collection.Limit,
					Expression:  expression,
					Embedding:   scene.SearchEmbedding,
					Extensions:  extensions,
				})
				for _, dep := range deps {
					scene.Dependencies = append(scene.Dependencies, render.Dependency(&dep))
				}
			}
			switch config.Layout.Type {
			case layout.Timeline:
				layout.LayoutTimeline(infos, config.Layout, &scene, imageSource)
			case layout.Album:
				layout.LayoutAlbum(infos, config.Layout, &scene, imageSource)
			case layout.Square:
				layout.LayoutSquare(&scene, imageSource)
			case layout.Wall:
				layout.LayoutWall(infos, config.Layout, &scene, imageSource)
			case layout.Map:
				layout.LayoutMap(infos, config.Layout, &scene, imageSource)
			case layout.Strip:
				layout.LayoutStrip(infos, config.Layout, &scene, imageSource)
			case layout.Flex:
				layout.LayoutFlex(infos, config.Layout, &scene, imageSource)
			default:
				layout.LayoutAlbum(infos, config.Layout, &scene, imageSource)
			}
		}

		if scene.RegionSource == nil {
			scene.RegionSource = &layout.PhotoRegionSource{
				Source: imageSource,
			}
		}
		finishedIndex := metrics.Elapsed("scene load " + config.Collection.Id)
		scene.BuildIndex()
		finishedIndex()
		scene.Dependencies = append(scene.Dependencies, config.Collection)
		scene.FileCount = len(scene.Photos)
		scene.Loading = false
		finished()
		log.Printf("photos %d, scene %.0f x %.0f\n", len(scene.Photos), scene.Bounds.W, scene.Bounds.H)
	}()

	return &scene
}

func (source *SceneSource) getOldestScene() (totalSize int64, oldestScene *render.Scene) {
	totalSize = 0
	source.scenes.Range(func(_, value interface{}) bool {
		stored := value.(storedScene)
		totalSize += getSceneCost(stored.scene)
		if oldestScene == nil || stored.scene.CreatedAt.Before(oldestScene.CreatedAt) {
			oldestScene = stored.scene
		}
		return true
	})
	return totalSize, oldestScene
}

func (source *SceneSource) deleteScene(id string) {
	log.Printf("scene delete %v", id)
	source.scenes.Delete(id)
	source.sceneCache.Del(id)
}

func (source *SceneSource) pruneScenes() {
	for {
		totalSize, oldestScene := source.getOldestScene()
		if totalSize <= int64(source.maxSize) {
			break
		}
		if oldestScene != nil {
			source.deleteScene(oldestScene.Id)
		}
	}
}

func (source *SceneSource) GetSceneById(id string, imageSource *image.Source) *render.Scene {
	if scene, ok := source.sceneCache.Get(id); ok {
		scene.UpdateStaleness()
		return scene
	}

	stored, loaded := source.scenes.Load(id)
	if loaded {
		scene := stored.(storedScene).scene
		scene.UpdateStaleness()
		source.sceneCache.Set(id, scene, getSceneCost(scene))
		return scene
	}
	return nil
}

func sceneConfigEqual(a SceneConfig, b SceneConfig) bool {
	if a.Collection.Limit != b.Collection.Limit {
		return false
	}
	if a.Collection.IndexLimit != b.Collection.IndexLimit {
		return false
	}
	for _, dirA := range a.Collection.Dirs {
		found := false
		for _, dirB := range b.Collection.Dirs {
			if dirA == dirB {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if a.Layout.ViewportWidth != 0 &&
		b.Layout.ViewportWidth != 0 &&
		a.Layout.ViewportWidth != b.Layout.ViewportWidth {
		return false
	}

	if a.Layout.ViewportHeight != 0 &&
		b.Layout.ViewportHeight != 0 &&
		a.Layout.ViewportHeight != b.Layout.ViewportHeight {
		return false
	}

	if a.Layout.ImageHeight != 0 &&
		b.Layout.ImageHeight != 0 &&
		a.Layout.ImageHeight != b.Layout.ImageHeight {
		return false
	}

	if a.Layout.Tweaks != b.Layout.Tweaks {
		return false
	}

	if a.Scene.Search != b.Scene.Search {
		return false
	}

	if a.Layout.Type != "" &&
		b.Layout.Type != "" &&
		a.Layout.Type != b.Layout.Type {
		return false
	}

	if a.Layout.Order != b.Layout.Order {
		return false
	}

	return true
}

func (source *SceneSource) GetScenesWithConfig(config SceneConfig) []*render.Scene {
	scenes := make([]*render.Scene, 0)
	source.scenes.Range(func(_, value interface{}) bool {
		stored := value.(storedScene)
		if sceneConfigEqual(stored.config, config) {
			stored.scene.UpdateStaleness()
			scenes = append(scenes, stored.scene)
		}
		return true
	})
	return scenes
}

func (source *SceneSource) Add(config SceneConfig, imageSource *image.Source) *render.Scene {

	id := config.Scene.Id
	if id == "" {
		var err error
		id, err = gonanoid.Generate("6789BCDFGHJKLMNPQRTWbcdfghjkmnpqrtwz", 10)
		if err != nil {
			panic(err)
		}
	}

	source.pruneScenes()

	scene := source.loadScene(config, imageSource)
	scene.Id = id

	source.scenes.Store(scene.Id, storedScene{
		scene:  scene,
		config: config,
	})
	return scene
}
