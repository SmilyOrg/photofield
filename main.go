package main

import (
	"embed"
	"flag"
	"fmt"
	goimage "image"
	"image/draw"
	"image/png"
	"io/fs"
	"io/ioutil"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	"github.com/joho/godotenv"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	io_prometheus_client "github.com/prometheus/client_model/go"

	"photofield/internal/codec"
	"photofield/internal/collection"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/metrics"
	"photofield/internal/openapi"
	"photofield/internal/render"
	"photofield/internal/scene"
)

//go:embed defaults.yaml
var defaultsYaml []byte
var defaults AppConfig

//go:embed db/migrations
var migrations embed.FS

//go:embed fonts/Roboto/Roboto-Regular.ttf
var robotoRegular []byte

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "unknown"
)

var startupTime time.Time

var defaultSceneConfig scene.SceneConfig

var tileRequestConfig TileRequestConfig

var tilePools sync.Map
var imageSource *image.Source
var sceneSource *scene.SceneSource
var collections []collection.Collection

var indexTasks sync.Map
var loadMetaOffset int64
var loadColorOffset int64

var tileRequestsOut chan struct{}
var tileRequests []TileRequest
var tileRequestsMutex sync.Mutex

var httpLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: metrics.Namespace,
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

type Task struct {
	Id           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	CollectionId string `json:"collection_id"`
	Done         int    `json:"done"`
	Pending      int    `json:"pending,omitempty"`
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

func drawTile(c *canvas.Context, r *render.Render, scene *render.Scene, zoom int, x int, y int) {

	tileSize := float64(r.TileSize)
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

	scales := render.Scales{
		Pixel: scale,
		Tile:  1 / float64(tileSize),
	}

	c.ResetView()

	img := r.CanvasImage
	draw.Draw(img, img.Bounds(), &goimage.Uniform{canvas.White}, goimage.Point{}, draw.Src)

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)

	c.SetFillColor(canvas.Black)

	scene.Draw(r, c, scales, imageSource)

}

func getTilePool(config *render.Render) *sync.Pool {
	stored, ok := tilePools.Load(config.TileSize)
	if ok {
		return stored.(*sync.Pool)
	}
	pool := sync.Pool{
		New: func() interface{} {
			return goimage.NewRGBA(goimage.Rect(0, 0, config.TileSize, config.TileSize))
		},
	}
	stored, _ = tilePools.LoadOrStore(config.TileSize, &pool)
	return stored.(*sync.Pool)
}

func getTileImage(config *render.Render) (*goimage.RGBA, *canvas.Context) {
	pool := getTilePool(config)
	img := pool.Get().(*goimage.RGBA)
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func putTileImage(config *render.Render, img *goimage.RGBA) {
	pool := getTilePool(config)
	pool.Put(img)
}

func getCollectionById(id string) *collection.Collection {
	for i := range collections {
		if collections[i].Id == id {
			return &collections[i]
		}
	}
	return nil
}

func getIndexTask(collection *collection.Collection) Task {
	return Task{
		Type:         string(openapi.TaskTypeINDEX),
		Id:           fmt.Sprintf("index-%v", collection.Id),
		Name:         fmt.Sprintf("Indexing %v", collection.Name),
		CollectionId: collection.Id,
		Done:         0,
	}
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

func renderSample(config render.Render, scene *render.Scene) {
	log.Println("rendering sample")
	config.LogDraws = true

	image, context := getTileImage(&config)
	defer putTileImage(&config, image)
	config.CanvasImage = image

	drawFinished := metrics.ElapsedWithCount("draw", len(scene.Photos))
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
	sceneConfig.Layout.Type = layout.Type(data.Layout)
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
		sceneConfig.Layout.Type = layout.Type(*params.Layout)
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
		Items []*render.Scene `json:"items"`
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
	for i := range collections {
		collection := &collections[i]
		collection.UpdateStatus(imageSource)
	}
	respond(w, r, http.StatusOK, struct {
		Items []collection.Collection `json:"items"`
	}{
		Items: collections,
	})
}

func (*Api) GetCollectionsId(w http.ResponseWriter, r *http.Request, id openapi.CollectionId) {

	for _, collection := range collections {
		if collection.Id == string(id) {
			collection.UpdateStatus(imageSource)
			respond(w, r, http.StatusOK, collection)
			return
		}
	}

	problem(w, r, http.StatusNotFound, "Scene not found")
}

func gatherIntFromMetric(value *int, metric *io_prometheus_client.MetricFamily, name string) {
	if metric.Name == nil || metric.Type == nil || *metric.Name != name {
		return
	}
	switch *metric.Type {
	case io_prometheus_client.MetricType_GAUGE:
		m := metric.Metric[0]
		if m == nil {
			return
		}
		g := m.Gauge
		if g == nil {
			return
		}
		*value = int(*g.Value)
	case io_prometheus_client.MetricType_COUNTER:
		m := metric.Metric[0]
		if m == nil {
			return
		}
		c := m.Counter
		if c == nil {
			return
		}
		*value = int(*c.Value)
	default:
		panic("Unsupported gather metric type")
	}
}

func (*Api) GetTasks(w http.ResponseWriter, r *http.Request, params openapi.GetTasksParams) {

	tasks := make([]Task, 0)

	if params.Type == nil || *params.Type == openapi.TaskTypeINDEX {
		indexTasks.Range(func(key, value interface{}) bool {
			task := value.(Task)
			if params.CollectionId == nil || task.CollectionId == string(*params.CollectionId) {
				tasks = append(tasks, task)
			}
			return true
		})
	}

	loadMetaTask := Task{
		Type: string(openapi.TaskTypeLOADMETA),
		Id:   "load-meta",
		Name: "Loading metadata",
	}
	loadColorTask := Task{
		Type: string(openapi.TaskTypeLOADCOLOR),
		Id:   "load-color",
		Name: "Loading colors",
	}

	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		problem(w, r, http.StatusInternalServerError, "Unable to gather metrics")
		return
	}
	for _, metric := range metrics {
		gatherIntFromMetric(&loadMetaTask.Pending, metric, "pf_load_meta_pending")
		gatherIntFromMetric(&loadMetaTask.Done, metric, "pf_load_meta_done")
		gatherIntFromMetric(&loadColorTask.Pending, metric, "pf_load_color_pending")
		gatherIntFromMetric(&loadColorTask.Done, metric, "pf_load_color_done")
	}

	if loadMetaTask.Pending > 0 {
		offset := atomic.LoadInt64(&loadMetaOffset)
		loadMetaTask.Done -= int(offset)
		tasks = append(tasks, loadMetaTask)
	} else {
		atomic.StoreInt64(&loadMetaOffset, int64(loadMetaTask.Done))
	}
	if loadColorTask.Pending > 0 {
		offset := atomic.LoadInt64(&loadColorOffset)
		loadColorTask.Done -= int(offset)
		tasks = append(tasks, loadColorTask)
	} else {
		atomic.StoreInt64(&loadColorOffset, int64(loadColorTask.Done))
	}

	sort.Slice(tasks, func(i, j int) bool {
		a := tasks[i]
		b := tasks[j]
		return a.Id < b.Id
	})

	respond(w, r, http.StatusOK, struct {
		Items []Task `json:"items"`
	}{
		Items: tasks,
	})
}

func (*Api) PostTasks(w http.ResponseWriter, r *http.Request) {
	data := &openapi.PostTasksJSONBody{}
	if err := chirender.Decode(r, data); err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	collection := getCollectionById(string(data.CollectionId))
	if collection == nil {
		problem(w, r, http.StatusBadRequest, "Collection not found")
		return
	}

	switch data.Type {

	case openapi.TaskTypeINDEX:
		task := getIndexTask(collection)
		stored, loaded := indexTasks.LoadOrStore(collection.Id, task)
		task = stored.(Task)
		if loaded {
			respond(w, r, http.StatusConflict, task)
		} else {
			respond(w, r, http.StatusAccepted, task)
			indexCollection(collection)
		}

	case openapi.TaskTypeLOADMETA:
		imageSource.QueueMetaLoads(collection.GetIds(imageSource))
		task := Task{
			Id:           fmt.Sprintf("load-meta-%v", collection.Id),
			CollectionId: collection.Id,
			Name:         fmt.Sprintf("Loading metadata for %v", collection.Name),
		}
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeLOADCOLOR:
		imageSource.QueueColorLoads(collection.GetIds(imageSource))
		task := Task{
			Id:           fmt.Sprintf("load-color-%v", collection.Id),
			CollectionId: collection.Id,
			Name:         fmt.Sprintf("Loading colors for %v", collection.Name),
		}
		respond(w, r, http.StatusAccepted, task)

	default:
		problem(w, r, http.StatusBadRequest, "Unsupported task type")
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
		return
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

	img, context := getTileImage(&render)
	defer putTileImage(&render, img)
	render.CanvasImage = img
	render.Zoom = zoom
	drawTile(context, &render, scene, zoom, x, y)

	codec.EncodeJpeg(w, img)
}

func (*Api) GetScenesSceneIdRegions(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdRegionsParams) {

	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
		return
	}

	bounds := render.Rect{
		X: float64(params.X),
		Y: float64(params.Y),
		W: float64(params.W),
		H: float64(params.H),
	}

	regions := scene.GetRegions(&defaultSceneConfig.Render, bounds, params.Limit)

	respond(w, r, http.StatusOK, struct {
		Items []render.Region `json:"items"`
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

	path, err := imageSource.GetImagePath(image.ImageId(id))
	if err == image.ErrNotFound {
		problem(w, r, http.StatusNotFound, "File not found")
		return
	}

	http.ServeFile(w, r, path)
}

func (*Api) GetFilesIdOriginalFilename(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam, filename openapi.FilenamePathParam) {

	path, err := imageSource.GetImagePath(image.ImageId(id))
	if err == image.ErrNotFound {
		problem(w, r, http.StatusNotFound, "File not found")
		return
	}

	http.ServeFile(w, r, path)
}

func (*Api) GetFilesIdImageVariantsSizeFilename(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam, size openapi.SizePathParam, filename openapi.FilenamePathParam) {

	imagePath, err := imageSource.GetImagePath(image.ImageId(id))
	if err == image.ErrNotFound {
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

	videoPath, err := imageSource.GetImagePath(image.ImageId(id))
	if err == image.ErrNotFound {
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

	http.ServeFile(w, r, path)
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

type TileRequestConfig struct {
	Concurrency int  `json:"concurrency"`
	LogStats    bool `json:"log_stats"`
}

type AppConfig struct {
	Collections  []collection.Collection `json:"collections"`
	Layout       layout.Layout           `json:"layout"`
	Render       render.Render           `json:"render"`
	Media        image.Config            `json:"media"`
	TileRequests TileRequestConfig       `json:"tile_requests"`
}

func expandCollections(collections *[]collection.Collection) {
	expanded := make([]collection.Collection, 0)
	for _, collection := range *collections {
		if collection.ExpandSubdirs {
			expanded = append(expanded, collection.Expand()...)
		} else {
			expanded = append(expanded, collection)
		}
	}
	*collections = expanded
}

func indexCollections(collections *[]collection.Collection) (ok bool) {
	go func() {
		for _, collection := range *collections {
			counter := make(chan int, 10)
			go func(id string, counter chan int) {
				task := getIndexTask(&collection)
				for add := range counter {
					task.Done += add
					indexTasks.Store(id, task)
				}
				indexTasks.Delete(id)
			}(collection.Id, counter)
			for _, dir := range collection.Dirs {
				imageSource.IndexImages(dir, collection.IndexLimit, counter)
			}
			close(counter)
		}
	}()
	return true
}

func indexCollection(collection *collection.Collection) {
	counter := make(chan int, 10)
	go func() {
		task := getIndexTask(collection)
		for add := range counter {
			task.Done += add
			indexTasks.Store(collection.Id, task)
		}
		indexTasks.Delete(collection.Id)
	}()
	go func() {
		log.Printf("indexing %s\n", collection.Id)
		for _, dir := range collection.Dirs {
			log.Printf("indexing %s %s\n", collection.Id, dir)
			imageSource.IndexImages(dir, collection.IndexLimit, counter)
		}
		close(counter)
	}()
}

func loadConfiguration(path string) AppConfig {

	var appConfig AppConfig

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("unable to open %s, using defaults (%s)\n", path, err.Error())
		appConfig = defaults
	} else if err := yaml.Unmarshal(bytes, &appConfig); err != nil {
		log.Printf("unable to parse %s, using defaults (%s)\n", path, err.Error())
		appConfig = defaults
	} else if err := mergo.Merge(&appConfig, defaults); err != nil {
		panic("unable to merge configuration with defaults")
	}

	expandCollections(&appConfig.Collections)
	for i := range appConfig.Collections {
		collection := &appConfig.Collections[i]
		collection.GenerateId()
		collection.Layout = strings.ToUpper(collection.Layout)
		if collection.Layout == "" {
			collection.Layout = string(appConfig.Layout.Type)
		}
		if collection.Limit > 0 && collection.IndexLimit == 0 {
			collection.IndexLimit = collection.Limit
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

func loadEnv() {
	env := os.Getenv("PHOTOFIELD_ENV")
	if env == "" {
		env = "development"
	}

	godotenv.Load(".env." + env + ".local")
	if env == "test" {
		godotenv.Load(".env.local")
	}
	godotenv.Load(".env." + env)
	godotenv.Load() // The Original .env
}

type spaFs struct {
	root http.FileSystem
}

func (fs spaFs) Open(name string) (http.File, error) {
	f, err := fs.root.Open(name)
	if os.IsNotExist(err) {
		return fs.root.Open("index.html")
	}
	return f, err
}

func main() {

	startupTime = time.Now()

	versionPtr := flag.Bool("version", false, "print version and exit")
	vacuumPtr := flag.Bool("vacuum", false, "clean database for smaller size and better performance, and exit")

	flag.Parse()

	if *versionPtr {
		fmt.Printf("photofield %s, commit %s, built on %s by %s\n", version, commit, date, builtBy)
		return
	}

	log.Printf("photofield %s", version)

	loadEnv()

	if err := yaml.Unmarshal(defaultsYaml, &defaults); err != nil {
		panic(err)
	}

	dataDir, exists := os.LookupEnv("PHOTOFIELD_DATA_DIR")
	if !exists {
		dataDir = "."
	}
	configurationPath := filepath.Join(dataDir, "configuration.yaml")

	appConfig := loadConfiguration(configurationPath)
	appConfig.Media.DatabasePath = filepath.Join(dataDir, "photofield.cache.db")

	if len(appConfig.Collections) > 0 {
		defaultSceneConfig.Collection = appConfig.Collections[0]
	}
	collections = appConfig.Collections
	defaultSceneConfig.Layout = appConfig.Layout
	defaultSceneConfig.Render = appConfig.Render
	tileRequestConfig = appConfig.TileRequests

	imageSource = image.NewSource(appConfig.Media, migrations)
	defer imageSource.Close()

	if *vacuumPtr {
		err := imageSource.Vacuum()
		if err != nil {
			panic(err)
		}
		return
	}

	sceneSource = scene.NewSceneSource()

	fontFamily := canvas.NewFontFamily("Main")
	// fontFamily.Use(canvas.CommonLigatures)
	err := fontFamily.LoadFont(robotoRegular, canvas.FontRegular)
	if err != nil {
		panic(err)
	}

	defaultSceneConfig.Scene.Fonts = render.Fonts{
		Main:   *fontFamily,
		Header: fontFamily.Face(14.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Hour:   fontFamily.Face(24.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Debug:  fontFamily.Face(34.0, canvas.Black, canvas.FontRegular, canvas.FontNormal),
	}
	sceneSource.DefaultScene = defaultSceneConfig.Scene

	// addExampleScene()
	// renderSample(defaultSceneConfig.Config, sceneSource.GetScene(defaultSceneConfig, imageSource))

	addr := ":8080"

	apiPrefix, exists := os.LookupEnv("PHOTOFIELD_API_PREFIX")
	if !exists {
		apiPrefix = "/api"
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
		subfs, err := fs.Sub(StaticFs, "ui/dist")
		if err != nil {
			panic(err)
		}
		sfs := spaFs{
			root: http.FS(subfs),
		}
		server := http.FileServer(sfs)
		r.Handle("/*", server)
		msg = fmt.Sprintf("ui at %v, %s", addr, msg)
	}

	log.Println(msg)
	log.Fatal(http.ListenAndServe(addr, r))
}
