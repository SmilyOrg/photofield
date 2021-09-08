package main

import (
	"embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"sort"
	"sync"
	"time"

	"io"
	"log"
	"net/http"
	"os"

	_ "net/http/pprof"

	"github.com/felixge/fgprof"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	chirender "github.com/go-chi/render"
	"github.com/imdario/mergo"
	spa "github.com/roberthodgen/spa-server"

	_ "github.com/joho/godotenv/autoload"

	. "photofield/internal"
	openapi "photofield/internal/api"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	. "photofield/internal/layout"
	. "photofield/internal/storage"

	_ "github.com/mkevac/debugcharts"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed defaults.yaml
var defaultsYaml []byte
var defaults AppConfig

//go:embed db/migrations
var migrations embed.FS

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

var httpLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: MetricsNamespace,
	Name:      "http_latency",
}, []string{"path"})

func instrumentationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())

		startTime := time.Now()
		next.ServeHTTP(rw, r)
		latency := time.Since(startTime).Seconds()

		path := rctx.RoutePattern()
		httpLatency.WithLabelValues(path).Observe(latency)
	})
}

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

type Problem struct {
	Status int    `json:"status"`
	Title  string `json:"title"`
}

func (p Problem) Render(w http.ResponseWriter, r *http.Request) error {
	chirender.Status(r, p.Status)
	return nil
}

func problem(w http.ResponseWriter, r *http.Request, code int, message string) {
	chirender.Render(w, r, Problem{
		Status: code,
		Title:  message,
	})
}

func respond(w http.ResponseWriter, r *http.Request, code int, v interface{}) {
	chirender.Status(r, code)
	chirender.Respond(w, r, v)
}

func drawTile(c *canvas.Context, render *Render, scene *Scene, zoom int, x int, y int) {

	tileSize := float64(render.TileSize)
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

	img := render.CanvasImage
	draw.Draw(img, img.Bounds(), &image.Uniform{canvas.White}, image.Point{}, draw.Src)

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)

	c.SetFillColor(canvas.Black)

	scene.Draw(render, c, scales, imageSource)

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

func getCollectionById(id string) *Collection {
	for i := range collections {
		if collections[i].Id == id {
			return &collections[i]
		}
	}
	return nil
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

func IndexHandler(entrypoint string) func(w http.ResponseWriter, r *http.Request) {
	fn := func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, entrypoint)
	}
	return http.HandlerFunc(fn)
}

type Api struct{}

func (*Api) PostScenes(w http.ResponseWriter, r *http.Request) {
	data := &openapi.SceneParams{}
	if err := chirender.Decode(r, data); err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	sceneConfig := defaultSceneConfig
	sceneConfig.Layout.SceneWidth = float64(data.SceneWidth)
	sceneConfig.Layout.ImageHeight = float64(data.ImageHeight)
	sceneConfig.Layout.Type = LayoutType(data.Layout)
	collection := getCollectionById(string(data.CollectionId))
	if collection == nil {
		problem(w, r, http.StatusBadRequest, "Collection not found")
		return
	}
	sceneConfig.Collection = *collection

	scene := sceneSource.Add(sceneConfig, imageSource)

	respond(w, r, http.StatusAccepted, scene)
}

func (*Api) GetScenes(w http.ResponseWriter, r *http.Request, params openapi.GetScenesParams) {

	sceneConfig := defaultSceneConfig
	if params.SceneWidth != nil {
		sceneConfig.Layout.SceneWidth = float64(*params.SceneWidth)
	}
	if params.ImageHeight != nil {
		sceneConfig.Layout.ImageHeight = float64(*params.ImageHeight)
	}
	if params.Layout != nil {
		sceneConfig.Layout.Type = LayoutType(*params.Layout)
	}
	collection := getCollectionById(string(params.CollectionId))
	if collection == nil {
		problem(w, r, http.StatusBadRequest, "Collection not found")
		return
	}
	sceneConfig.Collection = *collection

	scenes := sceneSource.GetScenesWithConfig(sceneConfig)
	sort.Slice(scenes, func(i, j int) bool {
		a := scenes[i]
		b := scenes[j]
		return a.CreatedAt.After(b.CreatedAt)
	})

	respond(w, r, http.StatusOK, struct {
		Items []*Scene `json:"items"`
	}{
		Items: scenes,
	})
}

func (*Api) GetScenesId(w http.ResponseWriter, r *http.Request, id openapi.SceneId) {

	scene := sceneSource.GetSceneById(string(id), imageSource)
	if scene == nil {
		problem(w, r, http.StatusNotFound, "Scene not found")
		return
	}

	respond(w, r, http.StatusOK, scene)
}

func (*Api) GetCollections(w http.ResponseWriter, r *http.Request) {
	respond(w, r, http.StatusOK, struct {
		Items []Collection `json:"items"`
	}{
		Items: collections,
	})
}

func (*Api) GetCollectionsId(w http.ResponseWriter, r *http.Request, id openapi.CollectionId) {

	for _, collection := range collections {
		if collection.Id == string(id) {
			respond(w, r, http.StatusOK, collection)
			return
		}
	}

	problem(w, r, http.StatusNotFound, "Scene not found")
}

func (*Api) GetIndexTasks(w http.ResponseWriter, r *http.Request, params openapi.GetIndexTasksParams) {

	tasks := make([]IndexTask, 0)
	indexTasks.Range(func(key, value interface{}) bool {
		task := value.(IndexTask)
		if task.CollectionId == string(params.CollectionId) {
			tasks = append(tasks, task)
		}
		return true
	})

	respond(w, r, http.StatusOK, struct {
		Items []IndexTask `json:"items"`
	}{
		Items: tasks,
	})
}

func (*Api) PostIndexTasks(w http.ResponseWriter, r *http.Request) {

	data := &openapi.PostIndexTasksJSONBody{}
	if err := chirender.Decode(r, data); err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	collection := getCollectionById(string(data.CollectionId))
	if collection == nil {
		problem(w, r, http.StatusBadRequest, "Collection not found")
		return
	}

	task := IndexTask{
		CollectionId: string(data.CollectionId),
		Count:        0,
	}
	stored, loaded := indexTasks.LoadOrStore(collection.Id, task)
	task = stored.(IndexTask)
	if loaded {
		respond(w, r, http.StatusConflict, task)
	} else {
		respond(w, r, http.StatusAccepted, task)
		indexCollection(collection)
	}
}

func (*Api) GetScenesSceneIdTiles(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdTilesParams) {
	startTime := time.Now()

	if tileRequestConfig.Concurrency == 0 {
		GetScenesSceneIdTilesImpl(w, r, sceneId, params)
	} else {
		request := TileRequest{
			Request:  r,
			Response: w,
			Priority: GetTilesRequestPriority(params),
			Process:  make(chan struct{}),
			Done:     make(chan struct{}),
		}
		pushTileRequest(request)
		<-request.Process
		GetScenesSceneIdTilesImpl(w, r, sceneId, params)
		request.Done <- struct{}{}
	}

	endTime := time.Now()
	if tileRequestConfig.LogStats {
		millis := endTime.Sub(startTime).Milliseconds()
		fmt.Printf("%4d, %4d, %4d, %4d\n", GetTilesRequestPriority(params), startTime.Sub(startupTime).Milliseconds(), endTime.Sub(startupTime).Milliseconds(), millis)
	}
}

func GetTilesRequestPriority(params openapi.GetScenesSceneIdTilesParams) int8 {
	zoom := params.Zoom
	if zoom >= 0 && zoom < 100 {
		return int8(zoom)
	}
	return 100
}

func GetScenesSceneIdTilesImpl(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdTilesParams) {
	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
	}

	render := defaultSceneConfig.Render
	render.TileSize = params.TileSize
	if params.DebugOverdraw != nil {
		render.DebugOverdraw = *params.DebugOverdraw
	}
	if params.DebugThumbnails != nil {
		render.DebugThumbnails = *params.DebugThumbnails
	}

	zoom := params.Zoom
	x := int(params.X)
	y := int(params.Y)

	image, context := getTileImage(&render)
	defer putTileImage(&render, image)
	render.CanvasImage = image
	render.Zoom = zoom
	drawTile(context, &render, scene, zoom, x, y)

	imageSource.Coder.EncodeJpeg(w, image)
}

func (*Api) GetScenesSceneIdRegions(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdRegionsParams) {

	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
		return
	}

	bounds := Rect{
		X: float64(params.X),
		Y: float64(params.Y),
		W: float64(params.W),
		H: float64(params.H),
	}

	render := defaultSceneConfig.Render
	regions := scene.GetRegions(&render, bounds, params.Limit)

	respond(w, r, http.StatusOK, struct {
		Items []Region `json:"items"`
	}{
		Items: regions,
	})
}

func (*Api) GetScenesSceneIdRegionsId(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, id openapi.RegionId) {

	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
		return
	}

	region := scene.GetRegion(int(id))
	if region.Id == -1 {
		http.Error(w, "Region not found", http.StatusNotFound)
		return
	}

	respond(w, r, http.StatusOK, region)
}

func (*Api) GetFilesId(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam) {

	path, err := imageSource.GetImagePath(ImageId(id))
	if err == NotFoundError {
		problem(w, r, http.StatusNotFound, "File not found")
		return
	}

	http.ServeFile(w, r, path)
}

func (*Api) GetFilesIdOriginalFilename(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam, filename openapi.FilenamePathParam) {

	path, err := imageSource.GetImagePath(ImageId(id))
	if err == NotFoundError {
		problem(w, r, http.StatusNotFound, "File not found")
		return
	}

	http.ServeFile(w, r, path)
}

func (*Api) GetFilesIdImageVariantsSizeFilename(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam, size openapi.SizePathParam, filename openapi.FilenamePathParam) {

	imagePath, err := imageSource.GetImagePath(ImageId(id))
	if err == NotFoundError {
		problem(w, r, http.StatusNotFound, "Image not found")
		return
	}

	path := ""
	for i := range imageSource.Images.Thumbnails {
		thumbnail := imageSource.Images.Thumbnails[i]
		candidatePath := thumbnail.GetPath(imagePath)
		if !imageSource.Exists(candidatePath) {
			continue
		}
		if thumbnail.Name != string(size) {
			continue
		}
		path = candidatePath
	}

	if path == "" || !imageSource.Exists(path) {
		problem(w, r, http.StatusNotFound, "Thumbnail not found")
		return
	}

	http.ServeFile(w, r, path)
}

func (*Api) GetFilesIdVideoVariantsSizeFilename(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam, size openapi.SizePathParam, filename openapi.FilenamePathParam) {

	// if size == "thumb" {
	// 	size = "M"
	// }

	videoPath, err := imageSource.GetImagePath(ImageId(id))
	if err == NotFoundError {
		problem(w, r, http.StatusNotFound, "Video not found")
		return
	}

	path := ""
	for i := range imageSource.Videos.Thumbnails {
		thumbnail := imageSource.Videos.Thumbnails[i]
		candidatePath := thumbnail.GetPath(videoPath)
		if !imageSource.Exists(candidatePath) {
			continue
		}
		if size != "full" && thumbnail.Name != string(size) {
			continue
		}
		path = candidatePath
	}

	if path == "" || !imageSource.Exists(path) {
		problem(w, r, http.StatusNotFound, "Resized video not found")
		return
	}
}

func AddPrefix(prefix string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx := chi.RouteContext(r.Context())

			routePath := rctx.RoutePath
			if routePath == "" {
				if r.URL.RawPath != "" {
					routePath = r.URL.RawPath
				} else {
					routePath = r.URL.Path
				}
				rctx.RoutePath = prefix + routePath
			}

			next.ServeHTTP(w, r)
		})
	}
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
		collection := &appConfig.Collections[i]
		collection.GenerateId()
		if collection.Layout == "" {
			collection.Layout = string(appConfig.Layout.Type)
		}
	}

	for i := range appConfig.Media.Images.Thumbnails {
		appConfig.Media.Images.Thumbnails[i].Init()
	}
	for i := range appConfig.Media.Videos.Thumbnails {
		appConfig.Media.Videos.Thumbnails[i].Init()
	}

	return appConfig
}

func addExampleScene() {
	sceneConfig := defaultSceneConfig
	sceneConfig.Scene.Id = "Tqcqtc6h69"
	sceneConfig.Layout.SceneWidth = 800
	sceneConfig.Layout.ImageHeight = 200
	sceneConfig.Collection = *getCollectionById("vacation-photos")
	sceneSource.Add(sceneConfig, imageSource)
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

	imageSource = NewImageSource(appConfig.System, appConfig.Media, migrations)
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

	// addExampleScene()
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

	r := chi.NewRouter()

	// r.Use(middleware.Logger)
	r.Use(instrumentationMiddleware)
	r.Use(middleware.Recoverer)

	r.Route(apiPrefix, func(r chi.Router) {

		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			MaxAge:         300, // Maximum value not ignored by any of major browsers
		}))

		var api Api
		r.Mount("/", openapi.Handler(&api))
		r.Mount("/metrics", promhttp.Handler())
	})
	msg := fmt.Sprintf("api at %v%v", addr, apiPrefix)

	r.Mount("/debug", middleware.Profiler())
	r.Handle("/debug/fgprof", fgprof.Handler())

	if apiPrefix != "/" {
		r.Handle("/*", spa.SpaHandler("static", "index.html"))
		msg += fmt.Sprintf(", ui at %v", addr)
	}

	log.Println(msg)
	log.Fatal(http.ListenAndServe(addr, r))
}
