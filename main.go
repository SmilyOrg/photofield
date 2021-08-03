package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"math"
	"sync"
	"time"

	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	_ "embed"
	_ "net/http/pprof"

	"github.com/felixge/fgprof"
	"github.com/imdario/mergo"

	_ "github.com/joho/godotenv/autoload"

	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	. "photofield/internal/layout"
	. "photofield/internal/storage"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mkevac/debugcharts"
	spa "github.com/roberthodgen/spa-server"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed defaults.yaml
var defaultsYaml []byte
var defaults AppConfig

var startupTime time.Time

var defaultSceneConfig SceneConfig

var tileRequestConfig TileRequestConfig

var tilePools sync.Map
var imageSource *ImageSource
var sceneSource *SceneSource
var collections []Collection

var indexTasks sync.Map

var tileRequestsOut chan struct{}
var tileRequests []TileRequest
var tileRequestsMutex sync.Mutex

type IndexTask struct {
	CollectionId string `json:"collection_id"`
	Count        int    `json:"count"`
}

type TileWriter func(w io.Writer) error

const MAX_PRIORITY = math.MaxInt8

type TileRequest struct {
	Request  *http.Request
	Response http.ResponseWriter
	// 0 - highest priority
	// 127 - lowest priority
	Priority int8
	Process  chan struct{}
	Done     chan struct{}
}

func drawTile(c *canvas.Context, config *Render, scene *Scene, zoom int, x int, y int) {

	tileSize := float64(config.TileSize)
	zoomPower := 1 << zoom

	tx := float64(x) * tileSize
	ty := float64(zoomPower-1-y) * tileSize

	var scale float64
	if 1 < scene.Bounds.W/scene.Bounds.H {
		scale = tileSize / scene.Bounds.W
		tx += (scale*scene.Bounds.W - tileSize) * 0.5
	} else {
		scale = tileSize / scene.Bounds.H
		ty += (scale*scene.Bounds.H - tileSize) * 0.5
	}

	scale *= float64(zoomPower)

	scales := Scales{
		Pixel: scale,
		Tile:  1 / float64(tileSize),
	}

	c.ResetView()

	backgroundStyle := canvas.Style{
		FillColor: canvas.White,
	}
	c.RenderPath(canvas.Rectangle(tileSize, tileSize), backgroundStyle, c.View())

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)

	c.SetFillColor(canvas.Black)

	scene.Draw(config, c, scales, imageSource)

}

func getTilePool(config *Render) *sync.Pool {
	stored, ok := tilePools.Load(config.TileSize)
	if ok {
		return stored.(*sync.Pool)
	}
	pool := sync.Pool{
		New: func() interface{} {
			return image.NewRGBA(image.Rect(0, 0, config.TileSize, config.TileSize))
		},
	}
	stored, _ = tilePools.LoadOrStore(config.TileSize, &pool)
	return stored.(*sync.Pool)
}

func getTileImage(config *Render) (*image.RGBA, *canvas.Context) {
	pool := getTilePool(config)
	img := pool.Get().(*image.RGBA)
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func putTileImage(config *Render, img *image.RGBA) {
	pool := getTilePool(config)
	pool.Put(img)
}

func getTileSize(config *Render, query *url.Values) int {
	tileSizeQuery, err := strconv.Atoi(query.Get("tileSize"))
	if err == nil && tileSizeQuery > 0 {
		return tileSizeQuery
	}
	return config.TileSize
}

func indexTasksHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "GET" {
		collectionId := query.Get("collection_id")

		tasks := make([]IndexTask, 0)
		indexTasks.Range(func(key, value interface{}) bool {
			task := value.(IndexTask)
			if task.CollectionId == collectionId {
				tasks = append(tasks, task)
			}
			return true
		})
		err := json.NewEncoder(w).Encode(struct {
			Items []IndexTask `json:"items"`
		}{
			Items: tasks,
		})
		if err != nil {
			http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
			return
		}
	} else if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		var request struct {
			CollectionId string `json:"collection_id"`
		}
		var err error
		err = decoder.Decode(&request)
		if err != nil {
			http.Error(w, "Unable to decode body as JSON", http.StatusBadRequest)
			return
		}
		collection := getCollectionById(request.CollectionId)
		if collection == nil {
			http.Error(w, "Invalid collection_id", http.StatusBadRequest)
			return
		}
		task := IndexTask{
			CollectionId: request.CollectionId,
			Count:        0,
		}
		stored, loaded := indexTasks.LoadOrStore(collection.Id, task)
		task = stored.(IndexTask)
		if loaded {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusAccepted)
			indexCollection(collection)
		}
		err = json.NewEncoder(w).Encode(task)
		if err != nil {
			http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
			return
		}
	}
}

func scenesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scenes := []*Scene{scene}
	err = json.NewEncoder(w).Encode(scenes)
	if err != nil {
		http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
		return
	}
}

func filterCollections(collections []Collection, f func(Collection) bool) []Collection {
	filtered := make([]Collection, 0)
	for _, collection := range collections {
		if f(collection) {
			filtered = append(filtered, collection)
		}
	}
	return filtered
}

func getCollectionById(id string) *Collection {
	for i := range collections {
		if collections[i].Id == id {
			return &collections[i]
		}
	}
	return nil
}

func collectionsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	w.Header().Set("Content-Type", "application/json")

	name := query.Get("name")

	filtered := filterCollections(collections, func(collection Collection) bool {
		if name != "" && name != collection.Name {
			return false
		}
		return true
	})

	err := json.NewEncoder(w).Encode(filtered)
	if err != nil {
		http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
		return
	}
}

func collectionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)

	id := vars["id"]
	if id == "" {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	collection := getCollectionById(id)
	if collection == nil {
		http.Error(w, "Collection not found", http.StatusNotFound)
		return
	}

	err := json.NewEncoder(w).Encode(collection)
	if err != nil {
		http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
		return
	}
}

func tilesHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	request := TileRequest{
		Request:  r,
		Response: w,
		Priority: getTileRequestPriority(r),
		Process:  make(chan struct{}),
		Done:     make(chan struct{}),
	}
	if tileRequestConfig.Concurrency == 0 {
		tilesHandlerImpl(w, r)
	} else {
		pushTileRequest(request)
		<-request.Process
		tilesHandlerImpl(w, r)
		request.Done <- struct{}{}
	}
	endTime := time.Now()
	if tileRequestConfig.LogStats {
		millis := endTime.Sub(startTime).Milliseconds()
		fmt.Printf("%4d, %4d, %4d, %4d\n", request.Priority, startTime.Sub(startupTime).Milliseconds(), endTime.Sub(startupTime).Milliseconds(), millis)
	}
}

func getTileRequestPriority(r *http.Request) int8 {
	query := r.URL.Query()
	// score := 0.
	zoom, err := strconv.Atoi(query.Get("zoom"))
	if err == nil && zoom >= 0 && zoom < 100 {
		return int8(zoom)
		// score += 1 / (1 + (float64)(zoom))
	}
	return 100
}

func pushTileRequest(request TileRequest) {
	tileRequestsMutex.Lock()
	tileRequests = append(tileRequests, request)
	tileRequestsMutex.Unlock()
	tileRequestsOut <- struct{}{}
}

func popBestTileRequest() (bool, TileRequest) {
	<-tileRequestsOut

	var bestRequest TileRequest
	bestRequest.Priority = MAX_PRIORITY

	tileRequestsMutex.Lock()
	var bestIndex = -1
	for index, request := range tileRequests {
		if request.Priority < bestRequest.Priority {
			bestRequest = request
			bestIndex = index
		}
	}

	if bestIndex == -1 {
		tileRequestsMutex.Unlock()
		return false, bestRequest
	}

	tileRequests = append(tileRequests[:bestIndex], tileRequests[bestIndex+1:]...)
	tileRequestsMutex.Unlock()
	return true, bestRequest
}

func processTileRequests(concurrency int) {
	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				ok, request := popBestTileRequest()
				if !ok {
					panic("Mismatching tileRequestsIn and tileRequestsOut")
				}
				request.Process <- struct{}{}
				<-request.Done
			}
		}()
	}
}

func tilesHandlerImpl(w http.ResponseWriter, r *http.Request) {

	query := r.URL.Query()

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	render := defaultSceneConfig.Render
	render.TileSize = getTileSize(&render, &query)

	zoom, err := strconv.Atoi(query.Get("zoom"))
	if err != nil {
		http.Error(w, "Invalid zoom", http.StatusBadRequest)
		return
	}

	x, err := strconv.Atoi(query.Get("x"))
	if err != nil {
		http.Error(w, "Invalid x", http.StatusBadRequest)
		return
	}

	y, err := strconv.Atoi(query.Get("y"))
	if err != nil {
		http.Error(w, "Invalid y", http.StatusBadRequest)
		return
	}

	render.DebugOverdraw = query.Get("debugOverdraw") == "true"
	render.DebugThumbnails = query.Get("debugThumbnails") == "true"

	image, context := getTileImage(&render)
	defer putTileImage(&render, image)
	render.CanvasImage = image
	render.Zoom = zoom
	drawTile(context, &render, scene, zoom, x, y)
	imageSource.Coder.EncodeJpeg(w, image)
}

func regionsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	render := defaultSceneConfig.Render
	render.TileSize = getTileSize(&render, &query)

	x, err := strconv.ParseFloat(query.Get("x"), 64)
	if err != nil {
		http.Error(w, "Invalid x", http.StatusBadRequest)
		return
	}

	y, err := strconv.ParseFloat(query.Get("y"), 64)
	if err != nil {
		http.Error(w, "Invalid y", http.StatusBadRequest)
		return
	}

	width, err := strconv.ParseFloat(query.Get("w"), 64)
	if err != nil {
		http.Error(w, "Invalid width", http.StatusBadRequest)
		return
	}

	height, err := strconv.ParseFloat(query.Get("h"), 64)
	if err != nil {
		http.Error(w, "Invalid height", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	bounds := Rect{
		X: x,
		Y: y,
		W: width,
		H: height,
	}

	regions := scene.GetRegions(&render, bounds)

	json.NewEncoder(w).Encode(regions)
}

func regionHandler(w http.ResponseWriter, r *http.Request) {

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)

	id, err := strconv.ParseInt(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	region := scene.GetRegion(int(id))
	if region.Id == -1 {
		http.Error(w, "Id not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(region)
}

func fileHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	path, err := imageSource.GetImagePath(ImageId(id))
	if err == NotFoundError {
		http.Error(w, "Id not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, path)

}

func fileVideoHandler(w http.ResponseWriter, r *http.Request) {

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)

	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	if id < 0 || id >= len(scene.Photos) {
		http.Error(w, "Id out of bounds", http.StatusBadRequest)
		return
	}

	size := vars["size"]
	if size == "" {
		http.Error(w, "Invalid video size", http.StatusBadRequest)
		return
	}

	if size == "thumb" {
		size = "M"
	}

	photo := &scene.Photos[id]
	path := ""
	for i := range imageSource.Videos.Thumbnails {
		video := imageSource.Videos.Thumbnails[i]
		candidatePath := video.GetPath(photo.GetPath(imageSource))
		if !imageSource.Exists(candidatePath) {
			continue
		}
		if size != "full" && video.Name != size {
			continue
		}
		path = candidatePath
	}

	if path == "" || !imageSource.Exists(path) {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, path)

}

func fileThumbHandler(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)

	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid id", http.StatusBadRequest)
		return
	}

	size := vars["size"]
	if size == "" {
		http.Error(w, "Invalid size", http.StatusBadRequest)
		return
	}

	photoPath, err := imageSource.GetImagePath(ImageId(id))
	if err == NotFoundError {
		http.Error(w, "Id not found", http.StatusNotFound)
		return
	}

	path := ""
	for i := range imageSource.Images.Thumbnails {
		thumbnail := imageSource.Images.Thumbnails[i]
		candidatePath := thumbnail.GetPath(photoPath)
		if !imageSource.Exists(candidatePath) {
			continue
		}
		if thumbnail.Name != size {
			continue
		}
		path = candidatePath
	}

	if path == "" || !imageSource.Exists(path) {
		http.Error(w, "Thumbnail not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, path)
}

func renderSample(config Render, scene *Scene) {
	log.Println("rendering sample")
	config.LogDraws = true

	image, context := getTileImage(&config)
	defer putTileImage(&config, image)
	config.CanvasImage = image

	drawFinished := ElapsedWithCount("draw", len(scene.Photos))
	drawTile(context, &config, scene, 0, 0, 0)
	drawFinished()

	f, err := os.Create("out.png")
	if err != nil {
		panic(err)
	}
	png.Encode(f, image)
	f.Close()
}

func getSceneFromRequest(r *http.Request) (*Scene, error) {
	sceneConfig := defaultSceneConfig

	query := r.URL.Query()

	var err error
	var value string

	value = query.Get("sceneWidth")
	if value != "" {
		sceneConfig.Layout.SceneWidth, err = strconv.ParseFloat(value, 64)
		if err != nil || sceneConfig.Layout.SceneWidth <= 0 {
			return nil, errors.New("invalid sceneWidth")
		}
	}

	value = query.Get("imageHeight")
	if value != "" {
		sceneConfig.Layout.ImageHeight, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("invalid imageHeight")
		}
	}

	sceneConfig.Layout.Type = query.Get("layout")

	value = query.Get("collection")
	if value != "" {
		collection := getCollectionById(value)
		if collection == nil {
			return nil, errors.New("collection not found")
		}
		sceneConfig.Collection = *collection
	}

	if sceneConfig.Layout.Type == "" {
		sceneConfig.Layout.Type = sceneConfig.Collection.Layout
	}

	cacheKey := query.Get("cacheKey")

	// fmt.Printf("%.0f %.0f\n", sceneConfig.Layout.SceneWidth, sceneConfig.Layout.ImageHeight)

	// return getScene(sceneConfig), nil

	return sceneSource.GetScene(sceneConfig, imageSource, cacheKey), nil
}

type AppConfig struct {
	Collections  []Collection      `json:"collections"`
	Layout       Layout            `json:"layout"`
	Render       Render            `json:"render"`
	System       System            `json:"system"`
	Media        ImageSourceConfig `json:"media"`
	TileRequests TileRequestConfig `json:"tile_requests"`
}

func expandCollections(collections *[]Collection) {
	expanded := make([]Collection, 0)
	for _, collection := range *collections {
		if collection.ExpandSubdirs {
			expanded = append(expanded, collection.Expand()...)
		} else {
			expanded = append(expanded, collection)
		}
	}
	*collections = expanded
}

func indexCollections(collections *[]Collection) (ok bool) {
	counter := make(chan int, 10)
	go func() {
		task := IndexTask{
			CollectionId: "[all]",
			Count:        0,
		}
		for add := range counter {
			task.Count += add
			indexTasks.Store("[all]", task)
		}
		indexTasks.Delete("[all]")
	}()
	go func() {
		for _, collection := range *collections {
			for _, dir := range collection.Dirs {
				imageSource.IndexImages(dir, collection.ListLimit, counter)
			}
		}
		close(counter)
	}()
	return true
}

func indexCollection(collection *Collection) {
	counter := make(chan int, 10)
	go func() {
		task := IndexTask{
			CollectionId: collection.Id,
			Count:        0,
		}
		for add := range counter {
			task.Count += add
			indexTasks.Store(collection.Id, task)
		}
		indexTasks.Delete(collection.Id)
	}()
	go func() {
		log.Printf("indexing %s\n", collection.Id)
		for _, dir := range collection.Dirs {
			log.Printf("indexing %s %s\n", collection.Id, dir)
			imageSource.IndexImages(dir, collection.ListLimit, counter)
		}
		close(counter)
	}()
}

func loadConfiguration() AppConfig {

	var appConfig AppConfig

	filename := "data/configuration.yaml"
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Printf("unable to open %s, using defaults: %s\n", filename, err.Error())
		return defaults
	}

	if err := yaml.Unmarshal(bytes, &appConfig); err != nil {
		log.Printf("unable to parse %s, using defaults: %s\n", filename, err.Error())
		return defaults
	}

	if err := mergo.Merge(&appConfig, defaults); err != nil {
		panic("unable to merge configuration with defaults")
	}

	expandCollections(&appConfig.Collections)
	for i := range appConfig.Collections {
		appConfig.Collections[i].GenerateId()
	}

	for i := range appConfig.Media.Images.Thumbnails {
		appConfig.Media.Images.Thumbnails[i].Init()
	}
	for i := range appConfig.Media.Videos.Thumbnails {
		appConfig.Media.Videos.Thumbnails[i].Init()
	}

	return appConfig
}

func IndexHandler(entrypoint string) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, entrypoint)
	}
	return http.HandlerFunc(fn)
}

func main() {

	startupTime = time.Now()
	if err := yaml.Unmarshal(defaultsYaml, &defaults); err != nil {
		panic(err)
	}

	appConfig := loadConfiguration()

	if len(appConfig.Collections) > 0 {
		defaultSceneConfig.Collection = appConfig.Collections[0]
	}
	collections = appConfig.Collections
	defaultSceneConfig.Layout = appConfig.Layout
	defaultSceneConfig.Render = appConfig.Render
	tileRequestConfig = appConfig.TileRequests

	imageSource = NewImageSource(appConfig.System, appConfig.Media)
	defer imageSource.Close()
	sceneSource = NewSceneSource()

	indexCollections(&collections)

	fontFamily := canvas.NewFontFamily("Roboto")
	// fontFamily.Use(canvas.CommonLigatures)
	err := fontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
	if err != nil {
		panic(err)
	}
	err = fontFamily.LoadFontFile("fonts/Roboto/Roboto-Bold.ttf", canvas.FontBold)
	if err != nil {
		panic(err)
	}

	defaultSceneConfig.Scene.Fonts = Fonts{
		Header: fontFamily.Face(14.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Hour:   fontFamily.Face(24.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Debug:  fontFamily.Face(30.0, canvas.Black, canvas.FontRegular, canvas.FontNormal),
	}
	sceneSource.DefaultScene = defaultSceneConfig.Scene

	// renderSample(defaultSceneConfig.Config, sceneSource.GetScene(defaultSceneConfig, imageSource))

	addr := ":8080"

	apiPrefix, exists := os.LookupEnv("API_PREFIX")
	if !exists {
		apiPrefix = "/"
	}

	if tileRequestConfig.LogStats {
		log.Printf("logging tile request stats")
		fmt.Printf("priority,start,end,latency\n")
	}

	tileRequestsOut = make(chan struct{}, 10000)
	if tileRequestConfig.Concurrency > 0 {
		log.Printf("request concurrency %v", tileRequestConfig.Concurrency)
		processTileRequests(tileRequestConfig.Concurrency)
	}

	r := mux.NewRouter()

	api := r.PathPrefix(apiPrefix).Subrouter()
	api.Handle("/metrics", promhttp.Handler())
	api.HandleFunc("/index-tasks", indexTasksHandler)
	api.HandleFunc("/scenes", scenesHandler)
	api.HandleFunc("/collections", collectionsHandler)
	api.HandleFunc("/collections/{id}", collectionHandler)
	api.HandleFunc("/tiles", tilesHandler)
	api.HandleFunc("/regions", regionsHandler)
	api.HandleFunc("/regions/{id}", regionHandler)
	api.HandleFunc("/files/{id}", fileHandler)
	api.HandleFunc("/files/{id}/file/{filename}", fileHandler)
	api.HandleFunc("/files/{id}/thumb/{size}/{filename}", fileThumbHandler)
	api.HandleFunc("/files/{id}/video/{size}/{filename}", fileVideoHandler)

	r.PathPrefix("/").Handler(spa.SpaHandler("static", "index.html"))

	http.Handle("/", handlers.CORS()(r))

	http.DefaultServeMux.Handle("/debug/fgprof", fgprof.Handler())

	log.Println("listening on", addr+", "+addr+apiPrefix)
	log.Fatal(http.ListenAndServe(addr, nil))
}
