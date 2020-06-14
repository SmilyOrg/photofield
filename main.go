package main

import (
	"encoding/json"
	"image"
	"image/jpeg"
	"image/png"
	"sync"
	"time"

	// "image/png"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	_ "github.com/mkevac/debugcharts"

	. "photofield/internal"
	. "photofield/internal/display"
	. "photofield/internal/storage"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var mainScene Scene
var mainConfig Config

var imageSource *ImageSource

type TileWriter func(w io.Writer) error

type ImageConfigRef struct {
	config image.Config
}

type Metrics struct {
	ImageSource ImageSourceMetrics `json:"imageSource"`
}

func drawTile(c *canvas.Context, config *Config, scene *Scene, zoom int, x int, y int) {

	tileSize := float64(config.TileSize)
	zoomPower := 1 << zoom

	tx := float64(x) * tileSize
	ty := float64(zoomPower-1-y) * tileSize

	var scale float64
	if tileSize/tileSize < scene.Bounds.W/scene.Bounds.H {
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
	c.SetFillColor(canvas.White)
	c.DrawPath(0, 0, canvas.Rectangle(tileSize, tileSize))

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)

	c.SetFillColor(canvas.Black)

	scene.Draw(config, c, scales, imageSource)

}

func getTileImage(config *Config) (*image.RGBA, *canvas.Context) {
	img := image.NewRGBA(image.Rect(0, 0, config.TileSize, config.TileSize))
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func getTileSize(config *Config, query *url.Values) int {
	tileSizeQuery, err := strconv.Atoi(query.Get("tileSize"))
	if err == nil && tileSizeQuery > 0 {
		return tileSizeQuery
	}
	return config.TileSize
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	metrics := Metrics{
		ImageSource: imageSource.GetMetrics(),
	}
	err := json.NewEncoder(w).Encode(metrics)
	if err != nil {
		http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
		return
	}
}

func scenesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	scene := &mainScene
	scenes := []*Scene{scene}
	err := json.NewEncoder(w).Encode(scenes)
	if err != nil {
		http.Error(w, "Unable to encode to json", http.StatusInternalServerError)
		return
	}
}

func tilesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	config := mainConfig
	scene := mainScene

	config.TileSize = getTileSize(&config, &query)

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

	config.DebugOverdraw = query.Get("debugOverdraw") == "true"
	config.DebugThumbnails = query.Get("debugThumbnails") == "true"

	image, context := getTileImage(&config)
	scene.Canvas = image
	scene.Zoom = zoom
	drawTile(context, &config, &scene, zoom, x, y)
	// png.Encode(w, image)
	jpeg.Encode(w, image, &jpeg.Options{
		Quality: 80,
	})
}

func regionsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	scene := &mainScene
	config := mainConfig
	config.TileSize = getTileSize(&config, &query)

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

	regions := scene.GetRegions(&config, bounds)

	json.NewEncoder(w).Encode(regions)

	return
}

func regionHandler(w http.ResponseWriter, r *http.Request) {

	scene := &mainScene

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

	return
}

func main() {

	imageSource = NewImageSource()
	imageSource.Thumbnails = []Thumbnail{
		NewThumbnail(
			"S",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_S.jpg",
			FitInside,
			Size{X: 120, Y: 120},
		),
		NewThumbnail(
			"SM",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_SM.jpg",
			FitOutside,
			Size{X: 240, Y: 240},
		),
		// NewThumbnail(
		// 	"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_PREVIEW.jpg",
		// 	// Size{X: 480, Y: 320},
		// 	// Size{X: 480, Y: 480},
		// 	Size{X: 160, Y: 160},
		// ),
		NewThumbnail(
			"M",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_M.jpg",
			FitOutside,
			Size{X: 320, Y: 320},
		),
		NewThumbnail(
			"B",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_B.jpg",
			FitInside,
			Size{X: 640, Y: 640},
		),
		// NewThumbnail(
		// 	"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_L.jpg",
		// 	// Size{X: 640, Y: 427},
		// 	Size{X: 800, Y: 800},
		// ),
		NewThumbnail(
			"XL",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_XL.jpg",
			FitOutside,
			Size{X: 1280, Y: 1280},
		),
	}

	// maxPhotos := 1
	// maxPhotos := 20
	// maxPhotos := 100
	// maxPhotos := 500
	// maxPhotos := 1000
	// maxPhotos := 2500
	// maxPhotos := 5000
	// maxPhotos := 10000
	// maxPhotos := 15000
	// maxPhotos := 20000
	// maxPhotos := 50000
	// maxPhotos := 60000
	// maxPhotos := 75000
	maxPhotos := 100000
	// maxPhotos := 150000
	var photoDirs = []string{
		// "/mnt/d/photos/copy/USA 2018/Lumix/100_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/101_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/102_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/103_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/104_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/105_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/106_PANA",
		// "/mnt/d/photos/copy/USA 2018/",
		// "D:/photos/copy/USA 2018/",
		// "P:/Moments/USA 2018",
		// "P:/Moments/Cuba 2019",
		// "P:/Moments",
		// "V:/homes/Miha/Drive/Moments/Mobile/Samsung SM-G950F/Camera",
		// "V:/photo/Moments",
		// "P:/homes/Miha/Drive/Moments/Mobile/Samsung SM-G950F/Camera",
		"P:/homes/Miha/Drive/Moments",
		// "P:/photo/Moments",
		// "P:/photo/Moments/2020 Tierpark",
		// "\\\\Denkarium/photo/Moments",
	}

	scene := &mainScene

	fontFamily := canvas.NewFontFamily("Roboto")
	// fontFamily.Use(canvas.CommonLigatures)
	err := fontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
	if err != nil {
		panic(err)
	}

	scene.Fonts = Fonts{
		Header: fontFamily.Face(96.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Hour:   fontFamily.Face(24.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Debug:  fontFamily.Face(64.0, canvas.Black, canvas.FontRegular, canvas.FontNormal),
	}

	log.Println("listing")
	preListing := time.Now()
	paths := make(chan string)
	wg := &sync.WaitGroup{}
	wg.Add(len(photoDirs))
	for _, photoDir := range photoDirs {
		go imageSource.ListImages(photoDir, maxPhotos, paths, wg)
	}
	go scene.AddPhotosFromPaths(paths)
	wg.Wait()
	close(paths)
	postListing := time.Now()
	listingElapsed := postListing.Sub(preListing).Milliseconds()
	log.Printf("listing %4d ms all, %4.2f ms / photo\n", listingElapsed, float64(listingElapsed)/float64(len(scene.Photos)))

	mainConfig.TileSize = 256
	mainConfig.MaxSolidPixelArea = 1000

	config := mainConfig
	config.LogDraws = true

	preLayout := time.Now()
	// LayoutSquare(scene, imageSource)
	// LayoutWall(&config, scene, imageSource)
	// LayoutTimeline(&config, scene, imageSource)
	LayoutTimelineEvents(&config, scene, imageSource)
	// LayoutCalendar(&config, scene, imageSource)
	postLayout := time.Now()
	layoutElapsed := postLayout.Sub(preLayout).Milliseconds()
	log.Printf("layout %4d ms all, %4.2f ms / photo\n", layoutElapsed, float64(layoutElapsed)/float64(len(scene.Photos)))

	log.Printf("scene %.0f %.0f\n", scene.Bounds.W, scene.Bounds.H)

	log.Println("rendering sample")
	image, context := getTileImage(&config)
	scene.Canvas = image
	preDraw := time.Now()
	drawTile(context, &config, scene, 0, 0, 0)
	postDraw := time.Now()
	drawElapsed := postDraw.Sub(preDraw).Milliseconds()
	log.Printf("draw %4d ms all, %4.2f ms / photo\n", drawElapsed, float64(drawElapsed)/float64(len(scene.Photos)))
	f, err := os.Create("out.png")
	if err != nil {
		panic(err)
	}
	png.Encode(f, image)
	f.Close()

	log.Printf("photos %d\n", len(scene.Photos))
	log.Println("serving")

	fs := http.FileServer(http.Dir("./static"))

	r := mux.NewRouter()
	r.HandleFunc("/metrics", metricsHandler)
	r.HandleFunc("/scenes", scenesHandler)
	r.HandleFunc("/tiles", tilesHandler)
	r.HandleFunc("/regions", regionsHandler)
	r.HandleFunc("/regions/{id}", regionHandler)
	r.PathPrefix("/").Handler(fs)
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
