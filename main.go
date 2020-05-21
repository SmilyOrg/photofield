package main

import (
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"time"

	// "image/png"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

func drawTile(c *canvas.Context, config *Config, scene *Scene, zoom int, x int, y int) {

	tileSize := float64(config.TileSize)
	zoomPower := 1 << zoom

	tx := float64(x) * tileSize
	ty := float64(zoomPower-1-y) * tileSize

	var scale float64
	if tileSize/tileSize < scene.Size.Width/scene.Size.Height {
		scale = tileSize / scene.Size.Width
		tx += (scale*scene.Size.Width - tileSize) * 0.5
	} else {
		scale = tileSize / scene.Size.Height
		ty += (scale*scene.Size.Height - tileSize) * 0.5
	}

	scale *= float64(zoomPower)

	scales := Scales{
		Pixel: scale,
		Tile:  1 / float64(tileSize),
	}

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)
	c.SetFillColor(canvas.White)
	c.DrawPath(0, 0, canvas.Rectangle(scene.Size.Width, -scene.Size.Height))

	c.SetFillColor(canvas.Black)

	scene.Draw(config, c, scales, imageSource)

}

func getTileImage(config *Config) (*image.RGBA, *canvas.Context) {
	img := image.NewRGBA(image.Rect(0, 0, config.TileSize, config.TileSize))
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func tilesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	config := mainConfig
	scene := mainScene

	tileSizeQuery, err := strconv.Atoi(query.Get("tileSize"))
	if err == nil && tileSizeQuery > 0 {
		config.TileSize = tileSizeQuery
	}

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

	bounds := Bounds{
		X: x,
		Y: y,
		W: width,
		H: height,
	}

	regions := mainScene.GetRegions(bounds)

	json.NewEncoder(w).Encode(regions)

	return
}

func main() {

	// go func() {
	// 	time.Sleep(20 * time.Second)
	// 	println("writing profile")
	// 	memprof, err := os.Create("mem.pprof")
	// 	if err != nil {
	// 		logrus.Fatal(err)
	// 	}
	// 	pprof.WriteHeapProfile(memprof)
	// 	memprof.Close()
	// }()

	imageSource = NewImageSource()

	maxPhotos := 10
	// maxPhotos := 20
	// maxPhotos := 100
	// maxPhotos := 500
	// maxPhotos := 1000
	// maxPhotos := 5000
	// maxPhotos := 20000
	// maxPhotos := 50000
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
		"P:/Moments",
	}

	scene := &mainScene

	log.Println("walking")
	lastLogTime := time.Now()
	for _, photoDir := range photoDirs {
		filepath.Walk(photoDir,
			func(path string, info os.FileInfo, err error) error {

				now := time.Now()
				if now.Sub(lastLogTime) > 1*time.Second {
					lastLogTime = now
					log.Printf("walking %d\n", len(scene.Photos))
				}

				if err != nil {
					return err
				}
				if strings.Contains(path, "@eaDir") {
					return filepath.SkipDir
				}
				if !strings.HasSuffix(strings.ToLower(path), ".jpg") {
					return nil
				}

				photo := Photo{}
				photo.SetImagePath(path)
				scene.Photos = append(scene.Photos, photo)

				if len(scene.Photos) >= maxPhotos {
					return errors.New("Skipping the rest")
				}
				return nil
			},
		)
	}

	mainConfig.TileSize = 256
	mainConfig.Thumbnails = []Thumbnail{
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_S.jpg",
			FitInside,
			// Size{Width: 120, Height: 80},
			Size{Width: 120, Height: 120},
		),
		// NewThumbnail(
		// 	"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_SM.jpg",
		// 	FitOutside,
		// 	// Size{Width: 480, Height: 320},
		// 	Size{Width: 240, Height: 240},
		// ),
		// NewThumbnail(
		// 	"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_PREVIEW.jpg",
		// 	// Size{Width: 480, Height: 320},
		// 	// Size{Width: 480, Height: 480},
		// 	Size{Width: 160, Height: 160},
		// ),
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_M.jpg",
			FitOutside,
			// Size{Width: 480, Height: 320},
			// Size{Width: 480, Height: 480},
			Size{Width: 320, Height: 320},
		),
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_B.jpg",
			FitInside,
			// Size{Width: 640, Height: 427},
			Size{Width: 640, Height: 640},
		),
		// NewThumbnail(
		// 	"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_L.jpg",
		// 	// Size{Width: 640, Height: 427},
		// 	Size{Width: 800, Height: 800},
		// ),
		NewThumbnail(
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_XL.jpg",
			FitOutside,
			// Size{Width: 1920, Height: 1280},
			// Size{Width: 1920, Height: 1920},
			Size{Width: 1280, Height: 1280},
		),
	}

	config := mainConfig
	config.LogDraws = true

	preLayout := time.Now()
	// LayoutSquare(scene, imageSource)
	LayoutWall(&config, scene, imageSource)
	// LayoutTimeline(&config, scene, imageSource)
	// LayoutCalendar(&config, scene, imageSource)
	postLayout := time.Now()
	layoutElapsed := postLayout.Sub(preLayout).Milliseconds()
	log.Printf("layout %4d ms all, %4.2f ms / photo\n", layoutElapsed, float64(layoutElapsed)/float64(len(scene.Photos)))

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
	http.Handle("/", fs)
	http.HandleFunc("/tiles", tilesHandler)
	http.HandleFunc("/regions", regionsHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
