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
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/imdario/mergo"
)

type AppConfig struct {
	Collections  []collection.Collection `json:"collections"`
	Layout       layout.Layout           `json:"layout"`
	Render       render.Render           `json:"render"`
	Media        image.Config            `json:"media"`
	AI           clip.AI                 `json:"ai"`
	Geo          geo.Config              `json:"geo"`
	Tags         tag.Config              `json:"tags"`
	TileRequests TileRequestConfig       `json:"tile_requests"`
}

var CONFIG_FILENAME = "configuration.yaml"

func watchConfig(dataDir string, callback func(init bool)) {
	w, err := fs.NewFileWatcher(filepath.Join(dataDir, CONFIG_FILENAME))
	if err != nil {
		log.Fatalln("Unable to watch config", err)
	}
	go func() {
		defer w.Close()
		for {
			<-w.Events
			callback(false)
		}
	}()
	callback(true)
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
	if err != nil {
		return nil, fmt.Errorf("unable to read %s: %w", path, err)
	} else if err := yaml.Unmarshal(bytes, &appConfig); err != nil {
		return nil, fmt.Errorf("unable to parse %s: %w", path, err)
	} else if err := mergo.Merge(&appConfig, defaults); err != nil {
		return nil, fmt.Errorf("unable to merge defaults: %w", err)
	}

	appConfig.Collections = expandCollections(appConfig.Collections)
	for i := range appConfig.Collections {
		collection := &appConfig.Collections[i]
		collection.GenerateId()
		collection.Layout = strings.ToUpper(collection.Layout)
		if collection.Limit > 0 && collection.IndexLimit == 0 {
			collection.IndexLimit = collection.Limit
		}
	}

	appConfig.Media.AI = appConfig.AI
	appConfig.Media.DataDir = dataDir
	appConfig.Tags.Enable = appConfig.Tags.Enable || appConfig.Tags.Enabled
	return &appConfig, nil
}

func expandCollections(collections []collection.Collection) []collection.Collection {
	expanded := make([]collection.Collection, 0, len(collections))
	for _, collection := range collections {
		if collection.ExpandSubdirs {
			expanded = append(expanded, collection.Expand()...)
		} else {
			expanded = append(expanded, collection)
		}
	}
	return expanded
}
