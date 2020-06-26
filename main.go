package main

import (
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"image/png"

	// "image/png"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"strconv"

	. "photofield/internal"
	. "photofield/internal/collection"
	. "photofield/internal/display"
	. "photofield/internal/layout"
	. "photofield/internal/storage"

	"github.com/gorilla/mux"
	_ "github.com/mkevac/debugcharts"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

var defaultSceneConfig SceneConfig

var mainSceneConfig SceneConfig

var imageSource *ImageSource
var sceneSource *SceneSource

type TileWriter func(w io.Writer) error

type ImageConfigRef struct {
	config image.Config
}

type Metrics struct {
	ImageSource ImageSourceMetrics `json:"imageSource"`
}

func drawTile(c *canvas.Context, config *RenderConfig, scene *Scene, zoom int, x int, y int) {

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

func getTileImage(config *RenderConfig) (*image.RGBA, *canvas.Context) {
	img := image.NewRGBA(image.Rect(0, 0, config.TileSize, config.TileSize))
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func getTileSize(config *RenderConfig, query *url.Values) int {
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

func tilesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := defaultSceneConfig.Config
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
	config.CanvasImage = image
	config.Zoom = zoom
	drawTile(context, &config, scene, zoom, x, y)
	// png.Encode(w, image)
	jpeg.Encode(w, image, &jpeg.Options{
		Quality: 80,
	})
}

func regionsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	scene, err := getSceneFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	config := defaultSceneConfig.Config
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

	return
}

func fileHandler(w http.ResponseWriter, r *http.Request) {

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

	photo := &scene.Photos[id]
	path := imageSource.GetImagePath(photo.Id)
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
	for i := range imageSource.Videos {
		video := imageSource.Videos[i]
		candidatePath := video.GetPath(imageSource.GetImagePath(photo.Id))
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

func renderSample(config RenderConfig, scene *Scene) {
	log.Println("rendering sample")
	config.LogDraws = true

	image, context := getTileImage(&config)
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
		if err != nil {
			return nil, errors.New("Invalid sceneWidth")
		}
	}

	value = query.Get("imageHeight")
	if value != "" {
		sceneConfig.Layout.ImageHeight, err = strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("Invalid imageHeight")
		}
	}

	// fmt.Printf("%.0f %.0f\n", sceneConfig.Layout.SceneWidth, sceneConfig.Layout.ImageHeight)

	// return getScene(sceneConfig), nil

	return sceneSource.GetScene(sceneConfig, imageSource), nil
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
	imageSource.Videos = []Thumbnail{
		NewThumbnail(
			"M",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_FILM_M.mp4",
			FitInside,
			Size{X: 120, Y: 120},
		),
		NewThumbnail(
			"H264",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_FILM_H264.mp4",
			OriginalSize,
			Size{},
		),
	}

	sceneSource = NewSceneSource()

	defaultSceneConfig.Collection = Collection{
		// ListLimit: 1,
		// ListLimit: 3,
		// ListLimit: 10,
		// ListLimit: 20,
		// ListLimit: 100,
		// ListLimit: 500,
		// ListLimit: 1000,
		// ListLimit: 2500,
		// ListLimit: 5000,
		// ListLimit: 10000,
		// ListLimit: 15000,
		// ListLimit: 20000,
		// ListLimit: 50000,
		// ListLimit: 60000,
		// ListLimit: 75000,
		// ListLimit: 100000,
		ListLimit: 200000,
		Dirs: []string{
			// "P:/homes/Miha/Drive/Moments/Mobile/Samsung SM-G950F/Camera",
			"P:/homes/Miha/Drive/Moments",
			"P:/photo/Moments",
			// "P:/photo/Moments/2020 Tierpark",
			// "P:/photo/Moments/Cuba 2019",
			// "P:/photo/Moments/2020 Usedom",
			// "P:/photo/Moments/USA 2018",
		},
	}

	// var photoDirs = []string{
	// var photoDirs = []string{
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
	// "P:/homes/Miha/Drive/Moments",
	// "P:/photo/Moments",
	// "P:/photo/Moments/2020 Tierpark",
	// "P:/photo/Moments/Cuba 2019",
	// "P:/photo/Moments/2020 Usedom",
	// "P:/photo/Moments/USA 2018",
	// "\\\\Denkarium/photo/Moments",
	// }

	// scene := &mainScene

	fontFamily := canvas.NewFontFamily("Roboto")
	// fontFamily.Use(canvas.CommonLigatures)
	err := fontFamily.LoadFontFile("fonts/Roboto/Roboto-Regular.ttf", canvas.FontRegular)
	if err != nil {
		panic(err)
	}

	defaultSceneConfig.Scene.Fonts = Fonts{
		Header: fontFamily.Face(14.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Hour:   fontFamily.Face(24.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal),
		Debug:  fontFamily.Face(30.0, canvas.Black, canvas.FontRegular, canvas.FontNormal),
	}
	sceneSource.DefaultScene = defaultSceneConfig.Scene

	defaultSceneConfig.Config = RenderConfig{
		TileSize:          256,
		MaxSolidPixelArea: 1000,
	}

	defaultSceneConfig.Layout = LayoutConfig{
		SceneWidth:  2000,
		ImageHeight: 160,
	}

	renderSample(defaultSceneConfig.Config, sceneSource.GetScene(defaultSceneConfig, imageSource))

	log.Println("serving")

	fs := http.FileServer(http.Dir("./static"))

	r := mux.NewRouter()
	r.HandleFunc("/metrics", metricsHandler)
	r.HandleFunc("/scenes", scenesHandler)
	r.HandleFunc("/tiles", tilesHandler)
	r.HandleFunc("/regions", regionsHandler)
	r.HandleFunc("/regions/{id}", regionHandler)
	r.HandleFunc("/files/{id}", fileHandler)
	r.HandleFunc("/files/{id}/video/{size}/{filename}", fileVideoHandler)
	r.PathPrefix("/").Handler(fs)
	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
