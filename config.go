package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"photofield/internal/clip"
	"photofield/internal/collection"
	"photofield/internal/fs"
	"photofield/internal/geo"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/render"
	"photofield/tag"

	"github.com/goccy/go-yaml"
	"github.com/imdario/mergo"
)

type AppConfig struct {
	Collections   []collection.Collection `json:"collections"`
	ExpandedPaths []string                `json:"-"`
	Layout        layout.Layout           `json:"layout"`
	Render        render.Render           `json:"render"`
	Media         image.Config            `json:"media"`
	AI            clip.AI                 `json:"ai"`
	Geo           geo.Config              `json:"geo"`
	Tags          tag.Config              `json:"tags"`
	TileRequests  TileRequestConfig       `json:"tile_requests"`
}

var CONFIG_FILENAME = "configuration.yaml"

func watchConfig(dataDir string, callback func(appConfig *AppConfig)) {
	w, err := fs.NewFileWatcher(filepath.Join(dataDir, CONFIG_FILENAME))
	if err != nil {
		log.Fatalln("Unable to watch config", err)
	}

	var expandWatcher *fs.Watcher
	var collectionsChanged chan fs.Event
	var appConfig *AppConfig
	reloadConfig := func() {
		appConfig, err = loadConfig(dataDir)
		if err != nil {
			log.Fatalln("Unable to load config", err)
		}
		expandWatcher.Close()
		collectionsChanged = make(chan fs.Event)
		if len(appConfig.ExpandedPaths) > 0 {
			expandWatcher, err = fs.NewPathsWatcher(appConfig.ExpandedPaths)
			if err != nil {
				log.Fatalln("Unable to watch expanded paths", err)
			}
			collectionsChanged = expandWatcher.Events
		}
		callback(appConfig)
	}

	reloadConfig()
	go func() {
		defer w.Close()
		for {
			select {
			case <-w.Events:
				log.Println("config changed, reloading")
			case e := <-collectionsChanged:
				switch e.Op {
				case fs.Update, fs.Rename:
					if info, err := os.Stat(e.Path); err != nil || !info.IsDir() {
						// Updated or renamed item was not a dir
						continue
					}
				case fs.Remove:
					removed := false
					for _, collection := range appConfig.Collections {
						for _, dir := range collection.Dirs {
							if dir == e.Path {
								removed = true
								break
							}
						}
						if removed {
							break
						}
					}
					if !removed {
						// Removed item was not a collection dir
						continue
					}
				default:
					continue
				}
				log.Println("collection changed, reloading")
			}
			reloadConfig()
		}
	}()
}

func initDefaults() {
	if err := yaml.Unmarshal(defaultsYaml, &defaults); err != nil {
		panic(err)
	}
}

func loadConfig(dataDir string) (*AppConfig, error) {
	path := filepath.Join(dataDir, CONFIG_FILENAME)

	var appConfig AppConfig

	log.Printf("config path %v", path)
	bytes, err := os.ReadFile(path)
	if err == nil {
		if err := yaml.Unmarshal(bytes, &appConfig); err != nil {
			return nil, fmt.Errorf("unable to parse %s: %w", path, err)
		} else if err := mergo.Merge(&appConfig, defaults); err != nil {
			return nil, fmt.Errorf("unable to merge defaults: %w", err)
		}
	} else {
		log.Printf("config read failed (using defaults) for %s: %v", path, err)
		appConfig = defaults
	}

	// Expand collections
	collections := make([]collection.Collection, 0, len(appConfig.Collections))
	expandedDirs := make(map[string]bool) // Track deduplicated dirs
	for _, collection := range appConfig.Collections {
		if collection.ExpandSubdirs {
			for _, dir := range collection.Dirs {
				expandedDirs[dir] = true
			}
			collections = append(collections, collection.Expand()...)
		} else {
			collections = append(collections, collection)
		}
	}
	appConfig.ExpandedPaths = make([]string, 0, len(expandedDirs))
	for dir := range expandedDirs {
		appConfig.ExpandedPaths = append(appConfig.ExpandedPaths, dir)
	}

	for i := range collections {
		collection := &collections[i]
		collection.MakeValid()
	}

	// Override earlier collection with the same ID
	collectionsMap := make(map[string]int)
	for i := 0; i < len(collections); i++ {
		if idx, exists := collectionsMap[collections[i].Id]; exists {
			collections[idx] = collections[i]
			collections = append(collections[:i], collections[i+1:]...)
			i--
			continue
		}
		collectionsMap[collections[i].Id] = i
	}
	appConfig.Collections = collections

	appConfig.Media.AI = appConfig.AI
	appConfig.Media.DataDir = dataDir
	appConfig.Tags.Enable = appConfig.Tags.Enable || appConfig.Tags.Enabled
	return &appConfig, nil
}
