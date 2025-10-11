package main

import (
	"context"
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
	"math"
	"math/rand"
	"mime"
	"net"
	"os/signal"
	"path"
	"regexp"
	"runtime"
	"runtime/trace"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
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
	"github.com/grafana/pyroscope-go"
	"github.com/hako/durafmt"
	"github.com/joho/godotenv"
	"github.com/lpar/gzipped"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	io_prometheus_client "github.com/prometheus/client_model/go"

	"photofield/internal/codec"
	"photofield/internal/collection"
	"photofield/internal/fs/rewrite"
	"photofield/internal/geo"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/metrics"
	"photofield/internal/openapi"
	"photofield/internal/render"
	"photofield/internal/scene"
	"photofield/internal/test"
	pfio "photofield/io"
	"photofield/io/bench"
	"photofield/tag"
)

//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.8.2 -generate=types,chi-server -package=openapi -o internal/openapi/api.gen.go api.yaml

//go:embed defaults.yaml
var defaultsYaml []byte
var defaults AppConfig

var tagsEnabled bool

//go:embed db/migrations
var migrations embed.FS

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

type TilePool struct {
	TileSize int
	ImageMem codec.ImageMem
}

var imageSource *image.Source
var globalGeo *geo.Geo
var sceneSource *scene.SceneSource
var collections []collection.Collection

var globalTasks sync.Map

var requestsOut chan struct{}
var requests []ApiRequest
var requestsMutex sync.Mutex

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
	Id           string        `json:"id"`
	Type         string        `json:"type"`
	Name         string        `json:"name"`
	CollectionId string        `json:"collection_id"`
	Done         int           `json:"done"`
	Pending      int           `json:"pending,omitempty"`
	Offset       int           `json:"-"`
	Queue        string        `json:"-"`
	completed    chan struct{} `json:"-"`
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

func (t *Task) Completed() <-chan struct{} {
	return t.completed
}

type TileWriter func(w io.Writer) error

const MAX_PRIORITY = math.MaxInt8

type ApiRequest struct {
	Request  *http.Request
	Response http.ResponseWriter
	// 0 - highest priority, 127 - lowest priority
	Priority int8
	Process  chan struct{}
	Done     chan struct{}
	Handler  func() // Function to execute for this request
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

func drawTile(ctx context.Context, c *canvas.Context, r *render.Render, scene *render.Scene, zoom int, x int, y int) {

	scales := render.Scales{
		Tile: 1 / float64(r.TileSize),
	}

	c.ResetView()

	img := r.CanvasImage
	draw.Draw(img, img.Bounds(), &goimage.Uniform{r.BackgroundColor}, goimage.Point{}, draw.Src)

	matrix, tileRect := scene.TileView(zoom, x, y, r.TileSize)
	c.SetView(matrix)
	r.TileRect = tileRect

	c.SetFillColor(canvas.Black)

	scene.Draw(ctx, r, c, scales, imageSource)
}

func getTilePool(config *render.Render) *sync.Pool {
	p := TilePool{
		TileSize: config.TileSize,
		ImageMem: config.ImageMem,
	}
	stored, ok := tilePools.Load(p)
	if ok {
		return stored.(*sync.Pool)
	}
	pool := sync.Pool{}
	switch p.ImageMem {
	case codec.ImageMemPaletted:
		pool.New = func() interface{} {
			return goimage.NewPaletted(
				goimage.Rect(0, 0, p.TileSize, p.TileSize),
				color.Palette{
					color.RGBA{0x00, 0x00, 0x00, 0x00},
					color.RGBA{0xFF, 0xFF, 0xFF, 0xFF},
				},
			)
		}
	case codec.ImageMemNRGBA:
		pool.New = func() interface{} {
			return goimage.NewNRGBA(
				goimage.Rect(0, 0, p.TileSize, p.TileSize),
			)
		}
	default:
		pool.New = func() interface{} {
			return goimage.NewRGBA(
				goimage.Rect(0, 0, p.TileSize, p.TileSize),
			)
		}
	}
	stored, _ = tilePools.LoadOrStore(p, &pool)
	return stored.(*sync.Pool)
}

func getTileImage(config *render.Render) (draw.Image, *canvas.Context) {
	pool := getTilePool(config)
	img := pool.Get().(draw.Image)
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func putTileImage(config *render.Render, img draw.Image) {
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

func newFileIndexTask(collection *collection.Collection) *Task {
	return &Task{
		Type:         string(openapi.TaskTypeINDEXFILES),
		Id:           fmt.Sprintf("index-files-%v", collection.Id),
		Name:         fmt.Sprintf("Indexing files %v", collection.Name),
		CollectionId: collection.Id,
		Done:         0,
		completed:    make(chan struct{}),
	}
}

func pushApiRequest(request ApiRequest) {
	requestsMutex.Lock()
	requests = append(requests, request)
	requestsMutex.Unlock()
	requestsOut <- struct{}{}
}

func popBestApiRequest() (bool, ApiRequest) {
	<-requestsOut

	var bestRequest ApiRequest
	bestRequest.Priority = MAX_PRIORITY

	requestsMutex.Lock()
	var bestIndex = -1
	for index, request := range requests {
		if request.Priority < bestRequest.Priority {
			bestRequest = request
			bestIndex = index
		}
	}

	if bestIndex == -1 {
		requestsMutex.Unlock()
		return false, bestRequest
	}

	requests = append(requests[:bestIndex], requests[bestIndex+1:]...)
	requestsMutex.Unlock()
	return true, bestRequest
}

func processTileRequests(concurrency int) {
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			for {
				ok, request := popBestApiRequest()
				if !ok {
					log.Printf("tile request worker %v exiting", i)
				}
				request.Process <- struct{}{}
				<-request.Done
			}
		}(i)
	}
}

func renderSample(config render.Render, scene *render.Scene) {
	log.Println("rendering sample")
	config.LogDraws = true

	image, canvas := getTileImage(&config)
	defer putTileImage(&config, image)
	config.CanvasImage = image

	drawFinished := metrics.ElapsedWithCount("draw", len(scene.Photos))
	drawTile(context.Background(), canvas, &config, scene, 0, 0, 0)
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
	sceneConfig.Collection = collection

	sceneConfig.Layout.ViewportWidth = float64(data.ViewportWidth)
	sceneConfig.Layout.ViewportHeight = float64(data.ViewportHeight)
	if data.Tweaks != nil {
		sceneConfig.Layout.Tweaks = string(*data.Tweaks)
	}
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
	}
	if params.Tweaks != nil {
		sceneConfig.Layout.Tweaks = string(*params.Tweaks)
	}
	collection := getCollectionById(string(params.CollectionId))
	if collection == nil {
		problem(w, r, http.StatusBadRequest, "Collection not found")
		return
	}
	sceneConfig.Collection = collection

	// Disregard viewport height for album and timeline layouts
	// as they are invariant to it
	switch sceneConfig.Layout.Type {
	case layout.Album, layout.Timeline:
		sceneConfig.Layout.ViewportHeight = 0
	}

	scenes := sceneSource.GetScenesWithConfig(sceneConfig)
	sort.Slice(scenes, func(i, j int) bool {
		a := scenes[i]
		b := scenes[j]
		return a.CreatedAt.After(b.CreatedAt)
	})

	if params.Limit != nil && int(*params.Limit) < len(scenes) {
		scenes = scenes[:int(*params.Limit)]
	}

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
		collection.UpdateIndexedAt(imageSource)
	}
	items := collections
	if items == nil {
		items = make([]collection.Collection, 0)
	}
	respond(w, r, http.StatusOK, struct {
		Items []collection.Collection `json:"items"`
	}{
		Items: items,
	})
}

func (*Api) GetCollectionsId(w http.ResponseWriter, r *http.Request, id openapi.CollectionId) {

	for _, collection := range collections {
		if collection.Id == string(id) {
			collection.UpdateIndexedAt(imageSource)
			collection.UpdateIndexedCount(imageSource)
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

	tasks := make([]*Task, 0)
	globalTasks.Range(func(key, value interface{}) bool {
		t := value.(*Task)

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
		Items []*Task `json:"items"`
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
		task := stored.(*Task)
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeINDEXCONTENTS:
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{
			Color:     true,
			Embedding: true,
		})
		stored, _ := globalTasks.Load("index-contents")
		task := stored.(*Task)
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeINDEXCONTENTSCOLOR:
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{
			Color: true,
		})
		stored, _ := globalTasks.Load("index-contents")
		task := stored.(*Task)
		respond(w, r, http.StatusAccepted, task)

	case openapi.TaskTypeINDEXCONTENTSAI:
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{
			Embedding: true,
		})
		stored, _ := globalTasks.Load("index-contents")
		task := stored.(*Task)
		respond(w, r, http.StatusAccepted, task)

	default:
		problem(w, r, http.StatusBadRequest, "Unsupported task type")
	}
}

func (*Api) GetCapabilities(w http.ResponseWriter, r *http.Request) {
	docsurl := os.Getenv("PHOTOFIELD_DOCS_URL")
	if docsurl == "" {
		docsurl = "/docs/usage"
	}
	capabilities := openapi.Capabilities{}
	if imageSource != nil {
		capabilities.Search.Supported = true
	}
	capabilities.Tags.Supported = tagsEnabled
	capabilities.Docs.Supported = docsurl != ""
	capabilities.Docs.Url = docsurl
	respond(w, r, http.StatusOK, capabilities)
}

func (*Api) GetScenesSceneIdTiles(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdTilesParams) {
	startTime := time.Now()

	if tileRequestConfig.Concurrency == 0 {
		GetScenesSceneIdTilesImpl(w, r, sceneId, params)
	} else {
		request := ApiRequest{
			Request:  r,
			Response: w,
			Priority: GetTilesRequestPriority(params),
			Process:  make(chan struct{}),
			Done:     make(chan struct{}),
			Handler: func() {
				GetScenesSceneIdTilesImpl(w, r, sceneId, params)
			},
		}
		pushApiRequest(request)
		<-request.Process
		request.Handler()
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

func decodeColor(s string) (color.Color, error) {
	if s == "transparent" {
		return color.Transparent, nil
	}
	c, err := hex.DecodeString(strings.TrimPrefix(s, "#"))
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{
		A: 0xFF,
		R: c[0],
		G: c[1],
		B: c[2],
	}, nil
}

func GetScenesSceneIdTilesImpl(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdTilesParams) {
	ctx, task := trace.NewTask(r.Context(), "GetScenesSceneIdTilesImpl")
	defer task.End()

	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
		return
	}

	incomplete := scene.Loading

	rn := defaultSceneConfig.Render
	rn.TileSize = params.TileSize
	if params.Sources != nil {
		rn.Sources = make(pfio.Sources, len(*params.Sources))
		for _, src := range imageSource.Sources {
			for i, name := range *params.Sources {
				if src.Name() == name {
					if rn.Sources[i] != nil {
						problem(w, r, http.StatusBadRequest, "Duplicate source")
						return
					}
					rn.Sources[i] = src
				}
			}
		}
		for _, src := range rn.Sources {
			if src == nil {
				problem(w, r, http.StatusBadRequest, "Unknown source")
				return
			}
		}
	}

	if params.SelectTag != nil {
		t, ok := imageSource.GetTagByName(string(*params.SelectTag))
		if !ok {
			problem(w, r, http.StatusBadRequest, "Unknown tag")
			return
		}
		rn.Selected = imageSource.GetTagImageIds(t.Id)
	}

	if params.DebugOverdraw != nil {
		rn.DebugOverdraw = *params.DebugOverdraw
	}
	if params.DebugThumbnails != nil {
		rn.DebugThumbnails = *params.DebugThumbnails
	}
	if params.QualityPreset != nil {
		switch *params.QualityPreset {
		case "HIGH":
			rn.QualityPreset = render.QualityPresetHigh
		default:
			rn.QualityPreset = render.QualityPresetFast
		}
	}

	zoom := params.Zoom
	x := int(params.X)
	y := int(params.Y)

	rn.BackgroundColor = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	if params.BackgroundColor != nil {
		var err error
		rn.BackgroundColor, err = decodeColor(string(*params.BackgroundColor))
		if err != nil {
			problem(w, r, http.StatusBadRequest, "Invalid background color")
			return
		}
	}

	rn.Color = color.RGBA{0x00, 0x00, 0x00, 0xFF}
	if params.Color != nil {
		var err error
		rn.Color, err = decodeColor(string(*params.Color))
		if err != nil {
			problem(w, r, http.StatusBadRequest, "Invalid background color")
			return
		}
	}

	if params.TransparencyMask != nil {
		rn.TransparencyMask = *params.TransparencyMask
	}
	if rn.TransparencyMask {
		rn.BackgroundColor = color.Transparent
	}

	// Determine output format based on Accept header and capabilities
	acceptHeader := r.Header.Get("Accept")

	if rn.TransparencyMask {
		acceptHeader = "image/png"
	}

	ranges, err := codec.ParseAccept(acceptHeader)
	if err != nil {
		problem(w, r, http.StatusBadRequest, "Invalid Accept header")
		return
	}

	var encoder codec.Encoder
	var mr codec.MediaRange
	var ok bool
	if rn.BackgroundColor == color.Transparent {
		encoder, mr, ok = ranges.AlphaEncoder()
	} else {
		encoder, mr, ok = ranges.FastestEncoder()
	}
	if !ok {
		encoder, mr, ok = ranges.FirstSupported()
	}
	if !ok {
		problem(w, r, http.StatusBadRequest, "No supported image format in Accept header")
		return
	}

	if rn.TransparencyMask {
		encoder.Mem = codec.ImageMemPaletted
	}

	img, context := getTileImage(&rn)
	defer putTileImage(&rn, img)

	rn.CanvasImage = img
	rn.Zoom = zoom
	trace.WithRegion(ctx, "drawTile", func() {
		drawTile(ctx, context, &rn, scene, zoom, x, y)
	})

	if incomplete {
		w.Header().Add("Cache-Control", "no-cache")
	} else {
		w.Header().Add("Cache-Control", "max-age=86400") // 1 day
	}

	w.Header().Add("Content-Type", encoder.ContentType)
	w.Header().Add("Vary", "Accept")

	quality := mr.QualityParam()
	if quality == 0 {
		switch mr.Subtype {
		case "webp":
			quality = 80
		default:
			quality = 80
		}
	}
	if rn.QualityPreset == render.QualityPresetHigh {
		quality = 100
	}

	err = encoder.Func(w, img, quality)
	if err != nil {
		log.Printf("Error encoding image as %s: %v", mr.String(), err)
		problem(w, r, http.StatusInternalServerError, "Error encoding image")
	}
}

func (*Api) GetScenesSceneIdFeatures(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdFeaturesParams) {
	if tileRequestConfig.Concurrency == 0 {
		GetScenesSceneIdFeaturesImpl(w, r, sceneId, params)
	} else {
		request := ApiRequest{
			Request:  r,
			Response: w,
			Priority: -10,
			Process:  make(chan struct{}),
			Done:     make(chan struct{}),
			Handler: func() {
				GetScenesSceneIdFeaturesImpl(w, r, sceneId, params)
			},
		}
		pushApiRequest(request)
		<-request.Process
		request.Handler()
		request.Done <- struct{}{}
	}
}

func GetScenesSceneIdFeaturesImpl(w http.ResponseWriter, r *http.Request, sceneId openapi.SceneId, params openapi.GetScenesSceneIdFeaturesParams) {
	scene := sceneSource.GetSceneById(string(sceneId), imageSource)
	if scene == nil {
		problem(w, r, http.StatusBadRequest, "Scene not found")
		return
	}

	_, tileRect := scene.TileView(params.Zoom, int(params.X), int(params.Y), 1)

	geojson := openapi.GeoJSON{
		Type:     "FeatureCollection",
		Features: make([]openapi.GeoJSONFeature, 0),
	}
	// for photo := range scene.GetVisiblePhotos(tileRect) {
	// 	info := photo.GetInfo(imageSource)
	// 	fileId := openapi.FileId(photo.Id)
	// 	color := openapi.Color(fmt.Sprintf("#%06x", info.Color&0xFFFFFF))
	// 	bounds := photo.Sprite.Rect
	// 	geojson.Features = append(geojson.Features,
	// 		openapi.GeoJSONFeature{
	// 			Type: "Feature",
	// 			Geometry: openapi.GeoJSONPolygon{
	// 				Type: "Polygon",
	// 				Coordinates: [][][]float32{
	// 					[][]float32{
	// 						[]float32{float32(bounds.X), float32(bounds.Y)},
	// 						[]float32{float32(bounds.X + bounds.W), float32(bounds.Y)},
	// 						[]float32{float32(bounds.X + bounds.W), float32(bounds.Y + bounds.H)},
	// 						[]float32{float32(bounds.X), float32(bounds.Y + bounds.H)},
	// 						[]float32{float32(bounds.X), float32(bounds.Y)},
	// 					},
	// 				},
	// 			},
	// 			Properties: openapi.GeoJSONProperties{
	// 				FileId: &fileId,
	// 				Color:  &color,
	// 			},
	// 		},
	// 	)
	// }

	// for _, text := range scene.Texts {
	// 	if !text.Sprite.Rect.IsVisible(tileRect) {
	// 		continue
	// 	}
	// 	bounds := text.Sprite.Rect
	// 	geojson.Features = append(geojson.Features,
	// 		openapi.GeoJSONFeature{
	// 			Type: "Feature",
	// 			Geometry: openapi.GeoJSONPoint{
	// 				Type:        "Point",
	// 				Coordinates: []float32{float32(bounds.X + bounds.W/2), float32(bounds.Y + bounds.H/2)},
	// 			},
	// 			Properties: openapi.GeoJSONProperties{
	// 				Text:   &text.Text,
	// 			},
	// 		},
	// 	)
	// }

	for _, photo := range scene.ClusterPhotos {
		if !photo.Sprite.Rect.IsVisible(tileRect) {
			continue
		}
		bounds := photo.Sprite.Rect
		fileId := openapi.FileId(photo.Id)
		geojson.Features = append(geojson.Features,
			openapi.GeoJSONFeature{
				Type: "Feature",
				Geometry: openapi.GeoJSONPoint{
					Type:        "Point",
					Coordinates: []float32{float32(bounds.X + bounds.W/2), float32(bounds.Y + bounds.H/2)},
				},
				Properties: openapi.GeoJSONProperties{
					FileId: &fileId,
				},
			},
		)
	}

	if scene.Loading {
		w.Header().Add("Cache-Control", "no-cache")
	} else {
		w.Header().Add("Cache-Control", "max-age=86400") // 1 day
	}

	respond(w, r, http.StatusOK, geojson)
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

	var regions []render.Region

	limit := 0
	if params.Limit != nil {
		limit = int(*params.Limit)
	}

	// Handle range-based region fetching
	if params.IdRange != nil {
		// Range requests must use minimal fields
		if params.Fields == nil || string(*params.Fields) != "(id,bounds)" {
			problem(w, r, http.StatusBadRequest, "id_range requires fields parameter to be '(id,bounds)'")
			return
		}
		var err error
		regions, err = handleRegionsByRange(scene, *params.IdRange, limit)
		if err != nil {
			problem(w, r, http.StatusBadRequest, err.Error())
			return
		}
	} else if params.FileId != nil {
		if params.X != nil || params.Y != nil || params.W != nil || params.H != nil {
			problem(w, r, http.StatusBadRequest, "file_id and bounds are mutually exclusive")
			return
		}
		// Check if minimal response is requested
		minimal := params.Fields != nil && string(*params.Fields) == "(id,bounds)"
		regions = getRegionsByFileId(scene, *params.FileId, limit, minimal)
	} else if params.Closest != nil && *params.Closest {
		if params.X == nil || params.Y == nil {
			problem(w, r, http.StatusBadRequest, "x and y required")
			return
		}
		if params.Limit == nil || *params.Limit != 1 {
			problem(w, r, http.StatusBadRequest, "limit must be set to 1")
			return
		}

		p := render.Point{
			X: float64(*params.X),
			Y: float64(*params.Y),
		}

		// Check if minimal response is requested
		minimal := params.Fields != nil && string(*params.Fields) == "(id,bounds)"
		region, ok := getRegionClosestTo(scene, p, minimal)
		if !ok {
			regions = []render.Region{}
		} else {
			regions = []render.Region{region}
		}
	} else {
		if params.X == nil || params.Y == nil || params.W == nil || params.H == nil {
			problem(w, r, http.StatusBadRequest, "bounds, file_id, or id_range required")
			return
		}

		bounds := render.Rect{
			X: float64(*params.X),
			Y: float64(*params.Y),
			W: float64(*params.W),
			H: float64(*params.H),
		}

		// Check if minimal response is requested
		minimal := params.Fields != nil && string(*params.Fields) == "(id,bounds)"
		regions = getRegionsFromBounds(scene, bounds, limit, minimal)
	}

	if scene.Loading {
		w.Header().Add("Cache-Control", "no-cache")
	} else {
		w.Header().Add("Cache-Control", "max-age=86400") // 1 day
	}

	respond(w, r, http.StatusOK, struct {
		Items []render.Region `json:"items"`
	}{
		Items: regions,
	})
}

func handleRegionsByRange(scene *render.Scene, rangeStr string, limit int) ([]render.Region, error) {
	if rangeStr == "" {
		return []render.Region{}, nil
	}

	// Parse range in format "start:end"
	parts := strings.Split(rangeStr, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format, expected 'start:end'")
	}

	startId, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	endId, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))

	if err1 != nil {
		return nil, fmt.Errorf("invalid start ID: %v", err1)
	}
	if err2 != nil {
		return nil, fmt.Errorf("invalid end ID: %v", err2)
	}
	if startId > endId {
		return nil, fmt.Errorf("start ID cannot be greater than end ID")
	}

	var regions []render.Region

	for id := startId; id <= endId; id++ {
		// Range requests always return minimal regions
		region := scene.GetRegionMinimal(id)

		if region.Id != 0 { // Assuming 0 means not found
			regions = append(regions, region)
		}

		// Apply limit if specified
		if limit > 0 && len(regions) >= limit {
			break
		}
	}

	return regions, nil
}

func getRegionsByFileId(scene *render.Scene, fileId openapi.FileId, limit int, minimal bool) []render.Region {
	if minimal {
		return scene.GetRegionsByImageIdMinimal(image.ImageId(fileId), limit)
	}
	return scene.GetRegionsByImageId(image.ImageId(fileId), limit)
}

func getRegionClosestTo(scene *render.Scene, point render.Point, minimal bool) (render.Region, bool) {
	if minimal {
		return scene.GetRegionClosestToMinimal(point)
	}
	return scene.GetRegionClosestTo(point)
}

func getRegionsFromBounds(scene *render.Scene, bounds render.Rect, limit int, minimal bool) []render.Region {
	if minimal {
		return scene.GetRegionsMinimal(bounds, limit)
	}
	return scene.GetRegions(bounds, limit)
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

func (*Api) GetTags(w http.ResponseWriter, r *http.Request, params openapi.GetTagsParams) {

	q := ""
	if params.Q != nil {
		q = string(*params.Q)
	}

	tags := make([]tag.Tag, 0)
	for t := range imageSource.ListTags(q, 100) {
		tags = append(tags, t)
	}

	respond(w, r, http.StatusOK, struct {
		Items []tag.Tag `json:"items"`
	}{
		Items: tags,
	})
}

func (*Api) GetTagsId(w http.ResponseWriter, r *http.Request, id openapi.TagIdPathParam) {
	tag, exists := imageSource.GetTagByName(string(id))
	if !exists {
		problem(w, r, http.StatusNotFound, "Tag not found")
		return
	}
	w.Header().Add("ETag", tag.ETag())
	respond(w, r, http.StatusOK, tag)
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

	tag, exists := imageSource.GetTagByName(t.Name)
	if !exists {
		problem(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	respond(w, r, http.StatusCreated, struct {
		Id openapi.TagId `json:"id"`
	}{
		Id: openapi.TagId(tag.Name),
	})
}

func (*Api) PostTagsIdFiles(w http.ResponseWriter, r *http.Request, id openapi.TagIdPathParam) {

	data := &openapi.TagFilesPost{}
	if err := chirender.Decode(r, data); err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	t, err := imageSource.GetOrCreateTagFromName(string(id))
	if err != nil {
		problem(w, r, http.StatusBadRequest, err.Error())
		return
	}

	ids := image.NewIds()
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

		photos := scene.GetVisiblePhotos(bounds)
		for p := range photos {
			ids.AddInt(int(p.Id))
		}
	} else if data.FileId != nil {
		ids.AddInt(int(*data.FileId))
	} else if data.TagId != nil {
		srct, err := imageSource.GetOrCreateTagFromName(string(*data.TagId))
		if err != nil {
			problem(w, r, http.StatusBadRequest, err.Error())
			return
		}
		ids = imageSource.GetTagImageIds(srct.Id)
	} else {
		problem(w, r, http.StatusBadRequest, "Either scene_id+bounds or file_id required")
		return
	}

	if ids.Len() > 0 {
		switch data.Op {
		case "ADD":
			t.UpdatedAt = imageSource.AddTagIds(t.Id, ids)
		case "SUBTRACT":
			t.UpdatedAt = imageSource.RemoveTagIds(t.Id, ids)
		case "INVERT":
			t.UpdatedAt = imageSource.InvertTagIds(t.Id, ids)
		default:
			problem(w, r, http.StatusBadRequest, "Invalid op")
			return
		}
	}

	respond(w, r, http.StatusOK, t)
}

func (*Api) GetTagsIdFilesTags(w http.ResponseWriter, r *http.Request, id openapi.TagIdPathParam) {

	t, ok := imageSource.GetTagByName(string(id))
	if !ok {
		problem(w, r, http.StatusNotFound, "Tag not found")
		return
	}

	count, ok := imageSource.GetTagFilesCount(t.Id)
	if !ok {
		problem(w, r, http.StatusInternalServerError, "Failed to count tag ids")
		return
	}

	tags := make([]tag.Tag, 0)
	for t := range imageSource.ListTagsOfTag(t.Id, 10) {
		tags = append(tags, t)
	}

	respond(w, r, http.StatusOK, struct {
		Items []tag.Tag `json:"items"`
		Count int       `json:"file_count"`
	}{
		Items: tags,
		Count: count,
	})
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

func indexCollection(collection *collection.Collection) (task *Task, existing bool) {
	task = newFileIndexTask(collection)
	stored, existing := globalTasks.LoadOrStore(task.Id, task)
	task = stored.(*Task)
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
		log.Printf("indexing files %s done\n", collection.Id)
		// imageSource.IndexAI(collection.Dirs, collection.IndexLimit)
		imageSource.IndexMetadata(collection.Dirs, collection.IndexLimit, image.Missing{})
		imageSource.IndexContents(collection.Dirs, collection.IndexLimit, image.Missing{})
		globalTasks.Delete(task.Id)
		close(counter)

		now := time.Now()
		collection.IndexedAt = &now
		collection.IndexedCount = task.Done
		close(task.completed)
	}()
	return
}

func addExampleScene() {
	sceneConfig := defaultSceneConfig
	sceneConfig.Scene.Id = "Tqcqtc6h69"
	sceneConfig.Layout.ViewportWidth = 1920
	sceneConfig.Layout.ViewportHeight = 1080
	sceneConfig.Layout.ImageHeight = 300
	sceneConfig.Layout.Type = layout.Flex
	sceneConfig.Layout.Order = layout.DateAsc
	sceneConfig.Collection = getCollectionById("vacation")
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

// benchmarkSources runs a benchmark on image sources
//
// It's not very usable right now as it doesn't use a representative sample of images,
// but it's a start.
func benchmarkSources(collection *collection.Collection, seed int64, sampleSize int, count int) {
	ids := make([]image.ImageId, 0)
	for id := range collection.GetIds(imageSource) {
		ids = append(ids, id)
	}
	randGen := rand.New(rand.NewSource(seed))
	randGen.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if len(ids) > sampleSize {
		ids = ids[:sampleSize]
	}
	samples := make([]bench.Sample, 0)
	for _, id := range ids {
		path, err := imageSource.GetImagePath(id)
		if err != nil {
			panic(err)
		}
		info := imageSource.GetInfo(id)
		samples = append(samples, bench.Sample{
			Id:   pfio.ImageId(id),
			Path: path,
			Size: pfio.Size{
				X: info.Width,
				Y: info.Height,
			},
		})
	}
	sources := imageSource.Sources
	bench.BenchmarkSources(seed, sources, samples, count)
}

func invalidateDirs(dirs []string) {
	now := time.Now()
	for i := range collections {
		collection := &collections[i]
		updated := false
		for _, dir := range dirs {
			for _, d := range collection.Dirs {
				if strings.HasPrefix(dir, d) {
					updated = true
					break
				}
			}
			if updated {
				break
			}
		}
		if updated {
			collection.InvalidatedAt = &now
		}
	}
}

func applyConfig(appConfig *AppConfig) {
	if globalGeo != nil {
		err := globalGeo.Close()
		if err != nil {
			log.Printf("unable to close geo: %v", err)
		}
		globalGeo = nil
	}

	if tileRequestConfig.Concurrency > 0 {
		close(requestsOut)
	}

	if len(appConfig.Collections) > 0 {
		defaultSceneConfig.Collection = &appConfig.Collections[0]
	}
	collections = appConfig.Collections
	defaultSceneConfig.Layout = appConfig.Layout
	defaultSceneConfig.Render = appConfig.Render
	tileRequestConfig = appConfig.TileRequests
	tagsEnabled = appConfig.Tags.Enable

	var err error
	globalGeo, err = geo.New(
		appConfig.Geo,
		GeoFs,
	)
	if err != nil {
		log.Printf("geo disabled: %v", err)
	} else {
		log.Printf("%v", globalGeo.String())
	}

	oldSource := imageSource
	imageSource = image.NewSource(appConfig.Media, migrations, globalGeo)
	if oldSource != nil {
		oldSource.Close()
	}

	imageSource.HandleDirUpdates(invalidateDirs)
	if tileRequestConfig.Concurrency > 0 {
		log.Printf("request concurrency %v", tileRequestConfig.Concurrency)
		requestsOut = make(chan struct{}, 10000)
		processTileRequests(tileRequestConfig.Concurrency)
	}

	extensions := strings.Join(appConfig.Media.ListExtensions, ", ")
	log.Printf("extensions %v", extensions)

	log.Printf("%v collections", len(collections))
	for i := range collections {
		collection := &collections[i]
		collection.UpdateIndexedAt(imageSource)
		if !appConfig.Media.SkipCollectionCounts {
			collection.UpdateIndexedCount(imageSource)
		}
		indexedAgo := "N/A"
		if collection.IndexedAt != nil {
			indexedAgo = durafmt.Parse(time.Since(*collection.IndexedAt)).LimitFirstN(1).String()
		}
		if appConfig.Media.SkipCollectionCounts {
			log.Printf("  %v indexed %v ago", collection.Name, indexedAgo)
		} else {
			log.Printf("  %v - %v files indexed %v ago", collection.Name, collection.IndexedCount, indexedAgo)
		}
	}
}

func listenForShutdown() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		log.Printf("shutdown requested")
		if imageSource != nil {
			imageSource.Shutdown()
		}
		os.Exit(0)
	}()
}

func createDummyEntries(count int, seed int64) {
	log.Printf("Starting dummy data generation...")
	start := time.Now()

	// Use the new WriteDummyFiles method
	err := imageSource.WriteDummyFiles(count, seed)
	if err != nil {
		log.Printf("Error generating dummy data: %v", err)
		return
	}

	elapsed := time.Since(start)
	log.Printf("Dummy data generation completed in %v (%.1f entries/sec)", elapsed, float64(count)/elapsed.Seconds())
}

// generateTestPhotos creates test images using the internal test package
func generateTestPhotos(count int, outputDir string, seed int64, name, widthsStr, heightsStr string) error {
	// Parse widths and heights from comma-separated strings
	widths, err := parseIntList(widthsStr)
	if err != nil {
		return fmt.Errorf("invalid widths: %w", err)
	}

	heights, err := parseIntList(heightsStr)
	if err != nil {
		return fmt.Errorf("invalid heights: %w", err)
	}

	// Create image specs with different combinations of widths and heights
	specs := make([]test.ImageSpec, count)
	for i := 0; i < count; i++ {
		width := widths[i%len(widths)]
		height := heights[i%len(heights)]

		specs[i] = test.ImageSpec{
			Width:  width,
			Height: height,
		}
	}

	// Create dataset
	dataset := test.TestDataset{
		Name:    name,
		Seed:    seed,
		Samples: 1, // One sample per spec
		Images:  specs,
	}

	// Generate the images
	images, err := test.GenerateTestDataset(outputDir, dataset)
	if err != nil {
		return fmt.Errorf("failed to generate test dataset: %w", err)
	}

	log.Printf("Generated %d test images in %s", len(images), outputDir)
	for _, img := range images {
		log.Printf("- %s (%dx%d)", img.Name, img.Spec.Width, img.Spec.Height)
	}

	return nil
}

// parseIntList parses a comma-separated string of integers
func parseIntList(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	result := make([]int, len(parts))

	for i, part := range parts {
		val, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("invalid integer '%s': %w", part, err)
		}
		result[i] = val
	}

	return result, nil
}

// detectEncoderSupport logs which encoders are available at startup
func detectEncoderSupport() {
	log.Printf("encoders %s", codec.FastestEncoders.String())
	log.Printf("encoders with alpha %s", codec.AlphaEncoders.String())
}

func main() {
	var err error

	startupTime = time.Now()

	log.SetFlags(
		0,
	)

	testing.Init()
	versionFlag := flag.Bool("version", false, "print version and exit")
	vacuumFlag := flag.Bool("vacuum", false, "clean database for smaller size and better performance, and exit")
	benchFlag := flag.Bool("bench", false, "benchmark sources and exit")
	benchCollectionId := flag.String("bench.collection", "vacation-photos", "id of the collection to benchmark")
	benchSeed := flag.Int64("bench.seed", 123, "seed for random number generator")
	benchSample := flag.Int("bench.sample", 10000, "number of images from the collection to use as a sample")
	dummyFlag := flag.Bool("dummy", false, "create dummy database entries and exit")
	dummyCount := flag.Int("dummy.count", 100000, "number of dummy entries to create")
	dummySeed := flag.Int64("dummy.seed", 42, "seed for random dummy data generation")

	// Photo generation flags
	genPhotosFlag := flag.Bool("gen-photos", false, "generate test photos and exit")
	genPhotosCount := flag.Int("gen-photos.count", 10, "number of test photos to generate")
	genPhotosOutput := flag.String("gen-photos.output", "testdata/e2e-photos", "output directory for generated photos")
	genPhotosSeed := flag.Int64("gen-photos.seed", 12345, "seed for random photo generation")
	genPhotosName := flag.String("gen-photos.name", "e2e-test", "dataset name for generated photos")
	genPhotosWidths := flag.String("gen-photos.widths", "100,150,200", "comma-separated list of image widths")
	genPhotosHeights := flag.String("gen-photos.heights", "75,100", "comma-separated list of image heights")

	// Scan collection flag
	scanFlag := flag.String("scan", "", "scan specified collection and exit")

	flag.Parse()

	if *benchFlag {
		log.SetOutput(os.Stderr)
	}

	if *versionFlag {
		fmt.Printf("photofield %s, commit %s, built on %s by %s\n", version, commit, date, builtBy)
		return
	}

	if *genPhotosFlag {
		log.Printf("generating %d test photos", *genPhotosCount)
		err := generateTestPhotos(*genPhotosCount, *genPhotosOutput, *genPhotosSeed, *genPhotosName, *genPhotosWidths, *genPhotosHeights)
		if err != nil {
			log.Fatalf("failed to generate test photos: %v", err)
		}
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

	initDefaults()
	dataDir, exists := os.LookupEnv("PHOTOFIELD_DATA_DIR")
	if !exists {
		dataDir = "."
	}

	sceneSource = scene.NewSceneSource()

	fontFamily := canvas.NewFontFamily("Main")
	// fontFamily.Use(canvas.CommonLigatures)
	err = fontFamily.LoadFont(robotoRegular, canvas.FontRegular)
	if err != nil {
		panic(err)
	}

	defaultSceneConfig.Scene.Fonts = render.Fonts{
		Main:   *fontFamily,
		Header: fontFamily.Face(70, canvas.Black, canvas.FontRegular, canvas.FontNormal),
		Hour:   fontFamily.Face(24, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Debug:  fontFamily.Face(34, canvas.Black, canvas.FontRegular, canvas.FontNormal),
	}
	sceneSource.DefaultScene = defaultSceneConfig.Scene

	listenForShutdown()

	detectEncoderSupport()

	watchConfig(dataDir, func(appConfig *AppConfig) {
		applyConfig(appConfig)
	})

	// config, err := loadConfig(dataDir)
	// if err != nil {
	// 	panic(err)
	// }
	// applyConfig(config)

	if *vacuumFlag {
		err := imageSource.Vacuum()
		if err != nil {
			panic(err)
		}
		imageSource.Close()
		return
	}

	if *benchFlag {
		log.Printf("benchmark sources")

		count := flag.Lookup("test.count").Value.(flag.Getter).Get().(uint)

		c := getCollectionById(*benchCollectionId)
		if c == nil {
			panic(fmt.Errorf("collection %v not found", *benchCollectionId))
		}
		benchmarkSources(c, *benchSeed, *benchSample, int(count))
		return
	}

	if *dummyFlag {
		log.Printf("creating %d dummy database entries with seed %d", *dummyCount, *dummySeed)
		createDummyEntries(*dummyCount, *dummySeed)
		return
	}

	if *scanFlag != "" {
		log.Printf("collection %s scan starting", *scanFlag)
		c := getCollectionById(*scanFlag)
		if c == nil {
			log.Fatalf("collection %v not found", *scanFlag)
		}

		// Perform the scan
		task, existing := indexCollection(c)
		if existing {
			log.Printf("collection %s scan already in progress", *scanFlag)
		} else {
			log.Printf("collection %s scan started", *scanFlag)
		}

		// Wait for the task to complete using the completion channel
		<-task.Completed()

		log.Printf("collection %s scan finished", *scanFlag)

		imageSource.WaitOnQueue()
		imageSource.Close()
		return
	}

	metadataTask := &Task{
		Type:  string(openapi.TaskTypeINDEXMETADATA),
		Id:    "index-metadata",
		Name:  "Indexing metadata",
		Queue: "index_metadata",
	}
	globalTasks.Store(metadataTask.Id, metadataTask)

	contentsTask := &Task{
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

	r := chi.NewRouter()

	// r.Use(middleware.Logger)
	r.Use(instrumentationMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

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

	r.Mount("/debug", middleware.Profiler())
	r.Handle("/debug/fgprof", fgprof.Handler())

	msg := ""
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

		uifs, err := fs.Sub(StaticFs, "ui/dist")
		if err != nil {
			panic(err)
		}
		uihandler := gzipped.FileServer(
			spaFs{
				root: http.FS(uifs),
			},
		)

		r.Route("/", func(r chi.Router) {
			r.Use(CacheControl())
			r.Use(IndexHTML())
			if StaticDocsPath != "" {
				docfs, err := fs.Sub(StaticDocsFs, StaticDocsPath)
				if err != nil {
					panic(err)
				}
				docspath := os.Getenv("PHOTOFIELD_DOCS_PATH")
				if docspath != "" {
					// rewriteDocs(docfs, docsurl)
					var err error
					docfs, err = rewrite.FS(
						docfs,
						[]string{
							"html",
							"css",
							"js",
						},
						`("|url\()(/docs/)`,
						`${1}`+docspath,
					)
					if err != nil {
						panic(err)
					}
				}
				dochandler := gzipped.FileServer(http.FS(docfs))
				r.HandleFunc("/docs/*", func(w http.ResponseWriter, r *http.Request) {
					if ext := path.Ext(r.URL.Path); ext == "" {
						r.URL.Path += ".html"
					}
					http.StripPrefix("/docs", dochandler).ServeHTTP(w, r)
				})
			}
			r.Handle("/*", uihandler)
		})
		msg = fmt.Sprintf("app running (api under %s)", apiPrefix)
	} else {
		msg = "app running (api only)"
	}

	// addExampleScene()

	log.Printf("")
	log.Println(msg)
	log.Fatal(listenAndServe(addr, r))
}

type listenUrl struct {
	local bool
	ipv6  bool
	url   string
}

func getListenUrls(addr net.Addr) ([]listenUrl, error) {
	var urls []listenUrl
	switch vaddr := addr.(type) {
	case *net.TCPAddr:
		if vaddr.IP.IsUnspecified() {
			ifaces, err := net.Interfaces()
			if err != nil {
				return urls, fmt.Errorf("unable to list interfaces: %v", err)
			}
			for _, i := range ifaces {
				addrs, err := i.Addrs()
				if err != nil {
					return urls, fmt.Errorf("unable to list addresses for %v: %v", i.Name, err)
				}
				for _, a := range addrs {
					switch v := a.(type) {
					case *net.IPNet:
						urls = append(urls, listenUrl{
							local: v.IP.IsLoopback(),
							ipv6:  v.IP.To4() == nil,
							url:   fmt.Sprintf("http://%v", net.JoinHostPort(v.IP.String(), strconv.Itoa(vaddr.Port))),
						})
					default:
						urls = append(urls, listenUrl{
							url: fmt.Sprintf("http://%v", v),
						})
					}
				}
			}
		} else {
			urls = append(urls, listenUrl{
				local: vaddr.IP.IsLoopback(),
				url:   fmt.Sprintf("http://%v", vaddr.AddrPort()),
			})
		}
	default:
		urls = append(urls, listenUrl{
			url: fmt.Sprintf("http://%v", addr),
		})
	}
	return urls, nil
}

func listenAndServe(addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	a := ln.Addr()
	urls, err := getListenUrls(a)
	if err != nil {
		return err
	}
	// Sort by ipv4 first, then local, then url
	sort.Slice(urls, func(i, j int) bool {
		if urls[i].ipv6 != urls[j].ipv6 {
			return !urls[i].ipv6
		}
		if urls[i].local != urls[j].local {
			return urls[i].local
		}
		return urls[i].url < urls[j].url
	})

	for _, url := range urls {
		prefix := "network"
		if url.local {
			prefix = "local"
		}
		log.Printf("  %-8s %s\n", prefix, url.url)
	}
	return srv.Serve(ln)
}
