package main

import (
	"embed"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	goimage "image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"io/ioutil"
	"math"
	"mime"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
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
	"github.com/hako/durafmt"
	"github.com/imdario/mergo"
	"github.com/joho/godotenv"
	"github.com/lpar/gzipped"
	"github.com/pyroscope-io/client/pyroscope"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	"github.com/goccy/go-yaml"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	io_prometheus_client "github.com/prometheus/client_model/go"

	"photofield/internal/clip"
	"photofield/internal/codec"
	"photofield/internal/collection"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/metrics"
	"photofield/internal/openapi"
	"photofield/internal/render"
	"photofield/internal/scene"
	pfio "photofield/io"
	"photofield/tag"
)

//go:embed defaults.yaml
var defaultsYaml []byte
var defaults AppConfig

//go:embed db/migrations
var migrations embed.FS

//go:embed db/migrations-thumbs
var migrationsThumbs embed.FS

//go:embed fonts/Roboto/Roboto-Regular.ttf
var robotoRegular []byte

var staticCacheRegex = regexp.MustCompile(`.+\.\w`)

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

var globalTasks sync.Map

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
	Offset       int    `json:"-"`
	Queue        string `json:"-"`
}

func (t *Task) Counter() chan<- int {
	counter := make(chan int, 10)
	go func() {
		for add := range counter {
			t.Done += add
		}
	}()
	return counter
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
	draw.Draw(img, img.Bounds(), &goimage.Uniform{r.BackgroundColor}, goimage.Point{}, draw.Src)

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

func newFileIndexTask(collection *collection.Collection) Task {
	return Task{
		Type:         string(openapi.TaskTypeINDEXFILES),
		Id:           fmt.Sprintf("index-files-%v", collection.Id),
		Name:         fmt.Sprintf("Indexing files %v", collection.Name),
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

	collection := getCollectionById(string(data.CollectionId))
	if collection == nil {
		problem(w, r, http.StatusBadRequest, "Collection not found")
		return
	}
	sceneConfig.Collection = *collection

	sceneConfig.Layout.ViewportWidth = float64(data.ViewportWidth)
	sceneConfig.Layout.ViewportHeight = float64(data.ViewportHeight)
	sceneConfig.Layout.ImageHeight = 0
	if data.ImageHeight != nil {
		sceneConfig.Layout.ImageHeight = float64(*data.ImageHeight)
	}
	if sceneConfig.Collection.Layout != "" {
		sceneConfig.Layout.Type = layout.Type(sceneConfig.Collection.Layout)
	}
	if data.Layout != "" {
		sceneConfig.Layout.Type = layout.Type(data.Layout)
	}
	if data.Sort != nil {
		sceneConfig.Layout.Order = layout.OrderFromSort(string(*data.Sort))
		if sceneConfig.Layout.Order == layout.None {
			problem(w, r, http.StatusBadRequest, "Invalid sort")
			return
		}
	}
	if data.Search != nil {
		sceneConfig.Scene.Search = string(*data.Search)
		if sceneConfig.Layout.Type != layout.Strip {
			sceneConfig.Layout.Type = layout.Search
		}
	}

	scene := sceneSource.Add(sceneConfig, imageSource)

	respond(w, r, http.StatusAccepted, scene)
}

func (*Api) GetScenes(w http.ResponseWriter, r *http.Request, params openapi.GetScenesParams) {

	sceneConfig := defaultSceneConfig
	if params.ViewportWidth != nil {
		sceneConfig.Layout.ViewportWidth = float64(*params.ViewportWidth)
	}
	if params.ViewportHeight != nil {
		sceneConfig.Layout.ViewportHeight = float64(*params.ViewportHeight)
	}
	if params.ImageHeight != nil {
		sceneConfig.Layout.ImageHeight = float64(*params.ImageHeight)
	}
	if params.Layout != nil {
		sceneConfig.Layout.Type = layout.Type(*params.Layout)
	}
	if params.Sort != nil {
		sceneConfig.Layout.Order = layout.OrderFromSort(string(*params.Sort))
		if sceneConfig.Layout.Order == layout.None {
			problem(w, r, http.StatusBadRequest, "Invalid sort")
			return
		}
	}
	if params.Search != nil {
		sceneConfig.Scene.Search = string(*params.Search)
		if sceneConfig.Layout.Type != layout.Strip {
			sceneConfig.Layout.Type = layout.Search
		}
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

	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		problem(w, r, http.StatusInternalServerError, "Unable to gather metrics")
		return
	}

	tasks := make([]Task, 0)
	globalTasks.Range(func(key, value interface{}) bool {
		t := value.(Task)

		add := true
		if params.Type != nil && t.Type != string(*params.Type) {
			add = false
		}
		if params.CollectionId != nil && t.CollectionId != string(*params.CollectionId) {
			add = false
		}

		if t.Queue != "" {
			for _, m := range metrics {
				gatherIntFromMetric(&t.Pending, m, fmt.Sprintf("pf_%s_pending", t.Queue))
				gatherIntFromMetric(&t.Done, m, fmt.Sprintf("pf_%s_done", t.Queue))
			}
			if t.Pending > 0 {
				t.Done -= t.Offset
			} else {
				t.Offset = t.Done
				globalTasks.Store(t.Id, t)
				add = false
			}
		}
		if add {
			tasks = append(tasks, t)
		}
		return true
	})

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

	case openapi.TaskTypeINDEXFILES:
		task, existing := indexCollection(collection)
		if existing {
			respond(w, r, http.StatusConflict, task)
		} else {
			respond(w, r, http.StatusAccepted, task)
		}

	case openapi.TaskTypeINDEXMETADATA:
		imageSource.IndexMetadata(collection.Dirs, collection.IndexLimit, image.Missing{
			Metadata: true,
		})
		stored, _ := globalTasks.Load("index-metadata")
		task := stored.(Task)
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeINDEXCONTENTS:
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{
			Color:     true,
			Embedding: true,
		})
		stored, _ := globalTasks.Load("index-contents")
		task := stored.(Task)
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeINDEXCONTENTSCOLOR:
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{
			Color: true,
		})
		stored, _ := globalTasks.Load("index-contents")
		task := stored.(Task)
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeINDEXCONTENTSAI:
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{
			Embedding: true,
		})
		stored, _ := globalTasks.Load("index-contents")
		task := stored.(Task)
		respond(w, r, http.StatusAccepted, task)

	default:
		problem(w, r, http.StatusBadRequest, "Unsupported task type")
	}
}

func (*Api) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	respond(w, r, http.StatusOK, openapi.Capabilities{
		Search: openapi.Capability{
			Supported: imageSource.AI.Available(),
		},
	})
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
	if params.Sources != nil {
		render.Sources = make(pfio.Sources, len(*params.Sources))
		for _, src := range imageSource.Sources {
			for i, name := range *params.Sources {
				if src.Name() == name {
					if render.Sources[i] != nil {
						problem(w, r, http.StatusBadRequest, "Duplicate source")
						return
					}
					render.Sources[i] = src
				}
			}
		}
		for _, src := range render.Sources {
			if src == nil {
				problem(w, r, http.StatusBadRequest, "Unknown source")
				return
			}
		}
	}

	if params.SelectTag != nil {
		t, err := tag.FromNameRev(string(*params.SelectTag))
		if err != nil {
			problem(w, r, http.StatusBadRequest, "Invalid tag id")
			return
		}
		id, ok := imageSource.GetTagId(t.Name)
		if !ok {
			problem(w, r, http.StatusBadRequest, "Unknown tag")
			return
		}
		render.Selected = imageSource.GetTagImageIds(id)
	}

	if params.DebugOverdraw != nil {
		render.DebugOverdraw = *params.DebugOverdraw
	}
	if params.DebugThumbnails != nil {
		render.DebugThumbnails = *params.DebugThumbnails
	}

	zoom := params.Zoom
	x := int(params.X)
	y := int(params.Y)
	render.BackgroundColor = color.White
	if params.BackgroundColor != nil {
		c, err := hex.DecodeString(strings.TrimPrefix(*params.BackgroundColor, "#"))
		if err != nil {
			problem(w, r, http.StatusBadRequest, "Invalid background color")
			return
		}
		render.BackgroundColor = color.RGBA{
			A: 0xFF,
			R: c[0],
			G: c[1],
			B: c[2],
		}
	}

	img, context := getTileImage(&render)
	defer putTileImage(&render, img)
	render.CanvasImage = img
	render.Zoom = zoom
	drawTile(context, &render, scene, zoom, x, y)

	w.Header().Add("Cache-Control", "max-age=86400") // 1 day
	codec.EncodeJpeg(w, img)
}

func (*Api) GetScenesSceneIdDates(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdDatesParams) {
	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
		return
	}

	minHeight := 1
	maxHeight := 10000

	if params.Height < minHeight {
		problem(w, r, http.StatusBadRequest, fmt.Sprintf("Minimum height is %v", minHeight))
		return
	}

	if params.Height > maxHeight {
		problem(w, r, http.StatusBadRequest, fmt.Sprintf("Maximum height is %v", maxHeight))
		return
	}

	timestamps := scene.GetTimestamps(params.Height, imageSource)
	w.Header().Add("Content-Type", "application/octet-stream")
	chirender.Status(r, http.StatusOK)
	binary.Write(w, binary.LittleEndian, timestamps)
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
	if region.Id <= 0 {
		http.Error(w, "Region not found", http.StatusNotFound)
		return
	}

	respond(w, r, http.StatusOK, region)
}

func (*Api) PostTags(w http.ResponseWriter, r *http.Request) {

	data := &openapi.TagsPost{}
	if err := chirender.Decode(r, data); err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if data.Selection == nil {
		problem(w, r, http.StatusBadRequest, "Only selection supported")
		return
	}

	if data.CollectionId == nil {
		problem(w, r, http.StatusBadRequest, "collection_id required")
		return
	}

	t, err := tag.NewSelection(string(*data.CollectionId))
	if err != nil {
		problem(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	imageSource.AddTag(t.Name)

	tag, exists := imageSource.GetTag(t.Name)
	if !exists {
		problem(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	respond(w, r, http.StatusCreated, struct {
		Id openapi.TagId `json:"id"`
	}{
		Id: openapi.TagId(tag.NameRev()),
	})
}

func (*Api) PostTagsIdFiles(w http.ResponseWriter, r *http.Request, id openapi.TagIdPathParam) {

	data := &openapi.TagFilesPost{}
	if err := chirender.Decode(r, data); err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	t, err := imageSource.GetTagFromNameRev(string(id))
	if err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	ids := make(chan image.ImageId, 100)
	if data.SceneId != nil && data.Bounds != nil {
		scene := sceneSource.GetSceneById(string(*data.SceneId), imageSource)
		if scene == nil {
			problem(w, r, http.StatusBadRequest, "Scene not found")
			return
		}

		bounds := render.Rect{
			X: float64(data.Bounds.X),
			Y: float64(data.Bounds.Y),
			W: float64(data.Bounds.W),
			H: float64(data.Bounds.H),
		}

		go func() {
			defer close(ids)
			photos := scene.GetVisiblePhotos(bounds)
			for p := range photos {
				ids <- image.ImageId(p.Id)
			}
		}()
	} else if data.FileId != nil {
		go func() {
			defer close(ids)
			ids <- image.ImageId(*data.FileId)
		}()
	} else {
		problem(w, r, http.StatusBadRequest, "Either scene_id+bounds or file_id required")
		return
	}

	var rev int
	switch data.Op {
	case "ADD":
		rev, err = imageSource.AddTagIds(t.Id, ids)
	case "SUBTRACT":
		rev, err = imageSource.RemoveTagIds(t.Id, ids)
	case "INVERT":
		rev, err = imageSource.InvertTagIds(t.Id, ids)
	}
	t.Revision = rev

	if err != nil {
		problem(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	respond(w, r, http.StatusOK, t)
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

func (*Api) GetFilesIdVariantsSizeFilename(w http.ResponseWriter, r *http.Request, id openapi.FileIdPathParam, size openapi.SizePathParam, filename openapi.FilenamePathParam) {
	imageSource.GetImageReader(image.ImageId(id), string(size), func(rs io.ReadSeeker, err error) {
		if err != nil {
			problem(w, r, http.StatusBadRequest, err.Error())
			return
		}
		http.ServeContent(w, r, string(filename), time.Time{}, rs)
	})
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
	AI           clip.AI                 `json:"ai"`
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

func indexCollection(collection *collection.Collection) (task Task, existing bool) {
	task = newFileIndexTask(collection)
	stored, existing := globalTasks.LoadOrStore(task.Id, task)
	task = stored.(Task)
	if existing {
		return
	}

	counter := task.Counter()

	go func() {
		log.Printf("indexing files %s\n", collection.Id)
		for _, dir := range collection.Dirs {
			log.Printf("indexing files %s dir %s\n", collection.Id, dir)
			imageSource.IndexFiles(dir, collection.IndexLimit, counter)
		}
		// imageSource.IndexAI(collection.Dirs, collection.IndexLimit)
		imageSource.IndexMetadata(collection.Dirs, collection.IndexLimit, image.Missing{})
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{})
		globalTasks.Delete(task.Id)
		close(counter)
	}()
	return
}

func loadConfiguration(path string) AppConfig {

	var appConfig AppConfig

	log.Printf("config path %v", path)
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
		if collection.Limit > 0 && collection.IndexLimit == 0 {
			collection.IndexLimit = collection.Limit
		}
	}

	appConfig.Media.AI = appConfig.AI

	return appConfig
}

func addExampleScene() {
	sceneConfig := defaultSceneConfig
	sceneConfig.Scene.Id = "Tqcqtc6h69"
	sceneConfig.Layout.ViewportWidth = 1920
	sceneConfig.Layout.ViewportHeight = 1080
	sceneConfig.Layout.ImageHeight = 300
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
	if os.IsNotExist(err) && !strings.HasSuffix(name, ".br") && !strings.HasSuffix(name, ".gz") {
		return fs.root.Open("index.html")
	}
	return f, err
}

func CacheControl() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if staticCacheRegex.MatchString(r.URL.Path) {
				w.Header().Set("Cache-Control", "max-age=31536000")
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func IndexHTML() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/") || len(r.URL.Path) == 0 {
				r.URL.Path = path.Join(r.URL.Path, "index.html")
			}
			next.ServeHTTP(w, r)
		})
	}
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

	if os.Getenv("PYROSCOPE_HOST") != "" {
		log.Printf("pyroscope enabled at %s", os.Getenv("PYROSCOPE_HOST"))

		// These 2 lines are only required if you're using mutex or block profiling
		// Read the explanation below for how to set these rates:
		runtime.SetMutexProfileFraction(5)
		runtime.SetBlockProfileRate(5)

		pyroscope.Start(pyroscope.Config{
			ApplicationName: "photofield",
			ServerAddress:   os.Getenv("PYROSCOPE_HOST"),
			Logger:          nil,
			AuthToken:       os.Getenv("PYROSCOPE_AUTH_TOKEN"),
			Tags:            map[string]string{"hostname": os.Getenv("HOSTNAME")},
			ProfileTypes: []pyroscope.ProfileType{
				// these profile types are enabled by default:
				pyroscope.ProfileCPU,
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileInuseSpace,

				// these profile types are optional:
				pyroscope.ProfileGoroutines,
				pyroscope.ProfileMutexCount,
				pyroscope.ProfileMutexDuration,
				pyroscope.ProfileBlockCount,
				pyroscope.ProfileBlockDuration,
			},
		})
	}

	if err := yaml.Unmarshal(defaultsYaml, &defaults); err != nil {
		panic(err)
	}

	dataDir, exists := os.LookupEnv("PHOTOFIELD_DATA_DIR")
	if !exists {
		dataDir = "."
	}
	configurationPath := filepath.Join(dataDir, "configuration.yaml")

	appConfig := loadConfiguration(configurationPath)
	appConfig.Media.DataDir = dataDir

	if len(appConfig.Collections) > 0 {
		defaultSceneConfig.Collection = appConfig.Collections[0]
	}
	collections = appConfig.Collections
	defaultSceneConfig.Layout = appConfig.Layout
	defaultSceneConfig.Render = appConfig.Render
	tileRequestConfig = appConfig.TileRequests

	imageSource = image.NewSource(appConfig.Media, migrations, migrationsThumbs)
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

	extensions := strings.Join(appConfig.Media.ListExtensions, ", ")
	log.Printf("extensions %v", extensions)

	log.Printf("%v collections", len(collections))
	for i := range collections {
		collection := &collections[i]
		collection.UpdateStatus(imageSource)
		indexedAgo := "N/A"
		if collection.IndexedAt != nil {
			indexedAgo = durafmt.Parse(time.Since(*collection.IndexedAt)).LimitFirstN(1).String()
		}
		log.Printf("  %v - %v files indexed %v ago", collection.Name, collection.IndexedCount, indexedAgo)
	}

	metadataTask := Task{
		Type:  string(openapi.TaskTypeINDEXMETADATA),
		Id:    "index-metadata",
		Name:  "Indexing metadata",
		Queue: "index_metadata",
	}
	globalTasks.Store(metadataTask.Id, metadataTask)

	contentsTask := Task{
		Type:  string(openapi.TaskTypeINDEXCONTENTS),
		Id:    "index-contents",
		Name:  "Indexing contents",
		Queue: "index_contents",
	}
	globalTasks.Store(contentsTask.Id, contentsTask)

	// renderSample(defaultSceneConfig.Config, sceneSource.GetScene(defaultSceneConfig, imageSource))

	addr, exists := os.LookupEnv("PHOTOFIELD_ADDRESS")
	if !exists {
		addr = ":8080"
	}

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

		allowedOrigins := os.Getenv("PHOTOFIELD_CORS_ALLOWED_ORIGINS")
		if allowedOrigins != "" {
			r.Use(cors.Handler(cors.Options{
				AllowedOrigins: strings.Split(allowedOrigins, ","),
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
				MaxAge:         300, // Maximum value not ignored by any of major browsers
			}))
		}

		var api Api
		r.Mount("/", openapi.Handler(&api))
		r.Mount("/metrics", promhttp.Handler())
	})
	msg := fmt.Sprintf("api at %v%v", addr, apiPrefix)

	r.Mount("/debug", middleware.Profiler())
	r.Handle("/debug/fgprof", fgprof.Handler())

	if apiPrefix != "/" {
		// Hardcode well-known mime types, see https://github.com/golang/go/issues/32350
		mime.AddExtensionType(".js", "text/javascript")
		mime.AddExtensionType(".css", "text/css")
		mime.AddExtensionType(".html", "text/html")
		mime.AddExtensionType(".woff", "font/woff")
		mime.AddExtensionType(".woff2", "font/woff2")
		mime.AddExtensionType(".png", "image/png")
		mime.AddExtensionType(".jpg", "image/jpg")
		mime.AddExtensionType(".jpeg", "image/jpeg")
		mime.AddExtensionType(".ico", "image/vnd.microsoft.icon")
		subfs, err := fs.Sub(StaticFs, "ui/dist")
		if err != nil {
			panic(err)
		}

		sfs := spaFs{
			root: http.FS(subfs),
		}

		server := gzipped.FileServer(sfs)

		r.Route("/", func(r chi.Router) {
			r.Use(CacheControl())
			r.Use(IndexHTML())
			r.Handle("/*", server)
		})
		msg = fmt.Sprintf("ui at %v, %s", addr, msg)
	}

	// addExampleScene()

	log.Println(msg)
	log.Fatal(http.ListenAndServe(addr, r))
}
