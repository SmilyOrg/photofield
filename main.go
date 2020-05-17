package main

import (
	"errors"
	"image"
	"image/jpeg"
	"image/png"

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
)

// var fontFamily *canvas.FontFamily
// var textFace canvas.FontFace

// var img image.Image
var mainScene Scene
var mainConfig Config

var imageSource *ImageSource

type TileWriter func(w io.Writer) error

type ImageConfigRef struct {
	config image.Config
}

// func (source *ImageSource) GetImageConfig(path string) *image.Config {
// 	configRef := &ImageConfigRef{}
// 	stored, loaded := source.imageConfigByPath.LoadOrStore(path, configRef)
// 	configRef = stored.(*ImageConfigRef)
// 	if !loaded {
// 		file, err := os.Open(path)
// 		if err != nil {
// 			panic(err)
// 		}
// 		defer file.Close()
// 		config, _, err := decoder.DecodeConfig(file)
// 		if err != nil {
// 			panic(err)
// 		}
// 		configRef.config = config
// 	}
// 	return &configRef.config
// }

// func (t *Transform) Push(c *canvas.Context) {
// 	c.Push()
// 	c.ComposeView(t.view)
// }

// func (t *Transform) Pop(c *canvas.Context) {
// 	c.Pop()
// }

// func (bitmap *Bitmap) ensureImage() {

// 	if bitmap.image != nil {
// 		return
// 	}

// 	bitmap.image = imageSource.GetImage(bitmap.path)

// file, err := os.Open(bitmap.path)
// if err != nil {
// 	panic(err)
// }

// image, err := jpeg.Decode(file)
// if err != nil {
// 	panic(err)
// }

// bitmap.image = &image
// }

func drawTile(c *canvas.Context, config *Config, scene *Scene, zoom int, x int, y int) {

	tileSize := float64(config.TileSize)
	zoomPower := 1 << zoom

	// println(zoomPower, x, y)
	// edgeTiles := scale

	tx := float64(x) * tileSize
	ty := float64(zoomPower-1-y) * tileSize

	// fitScale := scene.Size.Width / tileSize

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

	// +tileSize*float64(zoomPower)

	matrix := canvas.Identity.
		Translate(float64(-tx), float64(-ty+tileSize*float64(zoomPower))).
		Scale(float64(scale), float64(scale))

	c.SetView(matrix)
	c.SetFillColor(canvas.White)
	c.DrawPath(0, 0, canvas.Rectangle(scene.Size.Width, -scene.Size.Height))

	c.SetFillColor(canvas.Black)

	scene.Draw(config, c, scales, imageSource)

	// headerFace := fontFamily.Face(28.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	// textFace := fontFamily.Face(12.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)

	// drawText(c, 30.0, canvas.NewTextBox(headerFace, "Document Example", 0.0, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[0], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))

	// p := &canvas.Path{}

	// p.MoveTo(100, 100)
	// p := canvas.Circle(50)

	// c.DrawImage(0, 0, img, 5000)

	// for iy := 0; iy < 11; iy++ {
	// 	for ix := 0; ix < 11; ix++ {
	// 		c.DrawPath(0.1*float64(ix), 0.1*float64(iy), canvas.Circle(0.01))
	// 	}
	// }

	// gridNum := 10
	// for iy := 0; iy < gridNum; iy++ {
	// 	for ix := 0; ix < gridNum; ix++ {
	// 		x := float64(ix) / float64(gridNum-1) * scene.Size.Width
	// 		y := float64(iy) / float64(gridNum-1) * scene.Size.Height
	// 		c.DrawPath(x, y, canvas.Circle(2))
	// 	}
	// }

	// for iy := 0; iy < 11; iy++ {
	// 	for ix := 0; ix < 11; ix++ {
	// 		c.DrawPath(100*float64(ix), -100*float64(iy), canvas.Circle(5))
	// 	}
	// }

	// c.DrawPath(0.0*scene.Size.Width, -0.0*scene.Size.Height, canvas.Circle(10))
	// c.DrawPath(0.0*scene.Size.Width, -1.0*scene.Size.Height, canvas.Circle(10))
	// c.DrawPath(1.0*scene.Size.Width, -0.0*scene.Size.Height, canvas.Circle(10))
	// c.DrawPath(1.0*scene.Size.Width, -1.0*scene.Size.Height, canvas.Circle(10))

	// c.DrawPath(0.1, 0.1, canvas.Circle(0.1))
	// c.DrawPath(0.1, 0.9, canvas.Circle(0.1))
	// c.DrawPath(0.9, 0.1, canvas.Circle(0.1))
	// c.DrawPath(0.9, 0.9, canvas.Circle(0.1))

	// photo := scene.photos[4]
	// for i := range photo.bitmaps {
	// 	bitmap := &photo.bitmaps[i]
	// 	bitmap.sprite.transform.view = canvas.Identity.
	// 		Translate(10, -float64(i)*100).
	// 		Scale(0.01, 0.01)
	// 	bitmap.Draw(c, scales)
	// }

	// c.ComposeView(canvas.Identity.Scale(1, -1))

	// n := 50
	// cx := 0.1
	// for i := 0; i < n; i++ {
	// 	radius := math.Pow10(-1 - i)
	// 	cx += radius
	// 	c.DrawPath(cx, 0.5, canvas.Circle(radius))
	// 	cx += radius
	// }

	// headerFace := fontFamily.Face(28.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
	// drawText(c, 0.0, canvas.NewTextBox(headerFace, "Document Example", 0.0, 0.0, 1, 1, 0.0, 0.0))

	// c.DrawPath(0.2, 0.3, canvas.Circle(0.1))
	// c.DrawPath(0.3, 0.3, canvas.Circle(0.05))
	// c.DrawPath(0.4, 0.3, canvas.Circle(0.01))
	// c.DrawPath(0.5, 0.3, canvas.Circle(0.005))
	// c.DrawPath(0.6, 0.3, canvas.Circle(0.001))
	// c.DrawPath(0.7, 0.3, canvas.Circle(0.0005))

	// lenna, err := os.Open("../lenna.png")
	// if err != nil {
	// 	panic(err)
	// }
	// img, err := png.Decode(lenna)
	// if err != nil {
	// 	panic(err)
	// }
	// imgDPM := 15.0
	// imgWidth := float64(img.Bounds().Max.X) / imgDPM
	// imgHeight := float64(img.Bounds().Max.Y) / imgDPM
	// c.DrawImage(170.0-imgWidth, y-imgHeight, img, imgDPM)

	// imgWidth := 50.
	// imgHeight := 50.

	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[1], 140.0-imgWidth-10.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[2], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
	//drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[3], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
}

// func getTileCanvas(config *Config, scene *Scene, zoom int, x int, y int) *canvas.Canvas {
// 	c := canvas.New(float64(config.TileSize), float64(config.TileSize))
// 	ctx := canvas.NewContext(c)
// 	drawTile(ctx, config, scene, zoom, x, y)
// 	return c
// }

func getTileImage(config *Config) (*image.RGBA, *canvas.Context) {
	img := image.NewRGBA(image.Rect(0, 0, config.TileSize, config.TileSize))
	renderer := rasterizer.New(img, 1.0)
	return img, canvas.NewContext(renderer)
}

func tilesHandler(w http.ResponseWriter, r *http.Request) {
	// fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])

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

	// c := getTileCanvas(&config, &mainScene, zoom, x, y)
	// rasterizer.PNGWriter(1.0)(w, c)
	// rasterizer.PNGWriter(1.0)(w, c)
	image, context := getTileImage(&config)
	scene.Canvas = image
	drawTile(context, &config, &scene, zoom, x, y)
	// png.Encode(w, image)
	jpeg.Encode(w, image, &jpeg.Options{
		Quality: 80,
	})

	// rasterizer.Draw(c *Canvas, resolution DPMM)
	// c.WriteFile("out.png", rasterizer.PNGWriter(5.0))
	// getTilePngWriter()(w)
}

// var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	// flag.Parse()
	// if *cpuprofile != "" {
	// 	f, _ := os.Create(*cpuprofile)
	// 	pprof.StartCPUProfile(f)
	// 	defer pprof.StopCPUProfile()
	// }

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

	// fontFamily = canvas.NewFontFamily("sans")
	// fontFamily.Use(canvas.CommonLigatures)
	// if err := fontFamily.LoadLocalFont("sans", canvas.FontRegular); err != nil {
	// 	panic(err)
	// }
	// textFace = fontFamily.Face(48.0, canvas.Lightgray, canvas.FontRegular, canvas.FontNormal)

	// photo, err := os.Open("P1110271.JPG")
	// if err != nil {
	// 	panic(err)
	// }

	// image, err := jpeg.Decode(photo)
	// if err != nil {
	// 	panic(err)
	// }

	// photoCount := 697
	// maxPhotos := 10
	// maxPhotos := 100
	// maxPhotos := 500
	maxPhotos := 1000
	// maxPhotos := 20000
	// var photoDirs = "./photos"
	var photoDirs = []string{
		// "/mnt/d/photos/copy/USA 2018/Lumix/100_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/101_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/102_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/103_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/104_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/105_PANA",
		// "/mnt/d/photos/copy/USA 2018/Lumix/106_PANA",
		// "/mnt/d/photos/copy/USA 2018/",
		"D:/photos/copy/USA 2018/",
	}
	// var photoPath = "/mnt/p/Moments/USA 2018/Cybershot/100MSDCF"
	// var photoPath = "/mnt/p/Moments/USA 2018/Lumix/100_PANA"
	// var photoPath = "/mnt/d/photos/resized/USA 2018/Lumix/100_PANA/"
	// var photoFilePaths []string

	// scene.photos = make([]Photo, photoCount)

	scene := &mainScene

	log.Println("walking")
	// lastLogTime := time.Now()
	for _, photoDir := range photoDirs {
		filepath.Walk(photoDir,
			func(path string, info os.FileInfo, err error) error {

				// now := time.Now()
				// if now.Sub(lastLogTime) > 1*time.Second {
				// 	lastLogTime = now
				// 	log.Printf("walking %d\n", len(scene.Photos))
				// }

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

				// fmt.Printf("adding %s\n", path)
				// photoFilePaths = append(photoFilePaths, path)
				if len(scene.Photos) >= maxPhotos {
					return errors.New("Skipping the rest")
				}
				return nil
			},
		)
	}

	// if err != nil {
	// 	log.Println(err)
	// }

	// files, err := ioutil.ReadDir("./photos/Trip_Wuhletal")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for _, file := range files {
	// 	name := file.Name()
	// 	if strings.HasSuffix(strings.ToLower(name), ".jpg") {
	// 		photoPaths = append(photoPaths, name)
	// 	}
	// }

	// small := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_S.jpg", dir, filename),
	// }
	// photo.bitmaps = append(photo.bitmaps, small)

	// medium := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_M.jpg", dir, filename),
	// }
	// photo.bitmaps = append(photo.bitmaps, medium)

	// big := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_B.jpg", dir, filename),
	// }
	// photo.bitmaps = append(photo.bitmaps, big)

	// xl := Bitmap{
	// 	path: fmt.Sprintf("%s@eaDir/%s/SYNOPHOTO_THUMB_XL.jpg", dir, filename),
	// }

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

	// scene.Size = Size{width: 1000, height: 1000}
	/*
		for i := range scene.Photos {
			photo := &mainScene.Photos[i]
			size := photo.Original.GetSize(imageSource)
			for i := range mainConfig.Thumbnails {
				thumbnail := &mainConfig.Thumbnails[i]

				thumbFitted := Size{
					Width:  thumbnail.Size.Width,
					Height: thumbnail.Size.Height,
				}
				thumbWidth, thumbHeight := thumbnail.Size.Width, thumbnail.Size.Height
				thumbRatio := thumbWidth / thumbHeight
				originalWidth, originalHeight := float64(size.X), float64(size.Y)
				originalRatio := originalWidth / originalHeight
				switch thumbnail.SizeType {
				case FitInside:
					if thumbRatio < originalRatio {
						thumbFitted.Height = thumbWidth / originalRatio
					} else {
						thumbFitted.Width = thumbHeight * originalRatio
					}
				case FitOutside:
					if thumbRatio > originalRatio {
						thumbFitted.Height = thumbWidth / originalRatio
					} else {
						thumbFitted.Width = thumbHeight * originalRatio
					}
				}
				fittedWidth, fittedHeight := int(math.Round(thumbFitted.Width)), int(math.Round(thumbFitted.Height))

				path, err := thumbnail.GetPath(photo.Original.Path)
				if err != nil {
					panic(err)
				}
				thumbInfo := imageSource.GetImageInfo(path)

				match := fittedWidth == thumbInfo.Config.Width && fittedHeight == thumbInfo.Config.Height
				if !match {
					fmt.Printf("orig %4d %4d box %4d %4d fitted %4d %4d actual %4d %4d equal %v   %s\n", size.X, size.Y, int(thumbnail.Size.Width), int(thumbnail.Size.Height), fittedWidth, fittedHeight, thumbInfo.Config.Width, thumbInfo.Config.Height, match, photo.Original.Path)
				}
			}
		}

	*/

	// LayoutSquare(scene, imageSource)
	// LayoutTimeline(&config, scene, imageSource)
	LayoutCalendar(&config, scene, imageSource)

	// scene.Size = Size{width: 210, height: 297}
	// scene.Size = Size{width: 297, height: 210}
	// scene.photos = make([]Photo, photoCount)
	// for i := 0; i < photoCount; i++ {
	// 	col := i % cols
	// 	row := i / cols

	// 	path := photoFilePaths[i]

	// 	photo := &scene.photos[i]
	// 	photo.SetImagePath(path)
	// 	photo.Place((imageWidth+margin)*float64(1+col), (imageHeight+margin)*float64(1+row), imageWidth, imageHeight)
	// 	now := time.Now()
	// 	if now.Sub(lastLogTime) > 1*time.Second {
	// 		lastLogTime = now
	// 		log.Printf("placing %d / %d\n", i, photoCount)
	// 	}
	// }

	// c := canvas.New(200, 200)
	// ctx := canvas.NewContext(c)
	// draw(ctx)

	log.Println("rendering sample")

	// c := getTileCanvas(&config, scene, 0, 0, 0)
	// c.WriteFile("out.png", rasterizer.PNGWriter(1.0))
	image, context := getTileImage(&config)
	scene.Canvas = image
	drawTile(context, &config, scene, 0, 0, 0)
	f, err := os.Create("out.png")
	if err != nil {
		panic(err)
	}
	png.Encode(f, image)
	f.Close()

	log.Println("serving")

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/tiles", tilesHandler)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

var lorem = []string{
	`Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nulla malesuada fringilla libero vel ultricies. Phasellus eu lobortis lorem. Phasellus eu cursus mi. Sed enim ex, ornare et velit vitae, sollicitudin volutpat dolor. Sed aliquam sit amet nisi id sodales. Aliquam erat volutpat. In hac habitasse platea dictumst. Pellentesque luctus varius nibh sit amet porta. Vivamus tempus, enim ut sodales aliquet, magna massa viverra eros, nec gravida risus ipsum a erat. Etiam dapibus sem augue, at porta nisi dictum non. Vestibulum quis urna ut ligula dapibus mollis eu vel nisl. Vestibulum lorem dolor, eleifend lacinia fringilla eu, pulvinar vitae metus.`,
	`Morbi dapibus purus vel erat auctor, vehicula tempus leo maximus. Aenean feugiat vel quam sit amet iaculis. Fusce et justo nec arcu maximus porttitor. Cras sed aliquam ipsum. Sed molestie mauris nec dui interdum sollicitudin. Nulla id egestas massa. Fusce congue ante. Interdum et malesuada fames ac ante ipsum primis in faucibus. Praesent faucibus tellus eu viverra blandit. Vivamus mi massa, hendrerit in commodo et, luctus vitae felis.`,
	`Quisque semper aliquet augue, in dignissim eros cursus eu. Pellentesque suscipit consequat nibh, sit amet ultricies risus. Suspendisse blandit interdum tortor, consectetur tristique magna aliquet eu. Aliquam sollicitudin eleifend sapien, in pretium nisi. Sed tempor eleifend velit quis vulputate. Donec condimentum, lectus vel viverra pharetra, ex enim cursus metus, quis luctus est urna ut purus. Donec tempus gravida pharetra. Sed leo nibh, cursus at hendrerit at, ultricies a dui. Maecenas eget elit magna. Quisque sollicitudin odio erat, sed consequat libero tincidunt in. Nullam imperdiet, neque quis consequat pellentesque, metus nisl consectetur eros, ut vehicula dui augue sed tellus.`,
	//` Vivamus varius ex sed nisi vestibulum, sit amet tincidunt ante vestibulum. Nullam et augue blandit dolor accumsan tempus. Quisque at dictum elit, id ullamcorper dolor. Nullam feugiat mauris eu aliquam accumsan.`,
}

var y = 205.0

func drawText(c *canvas.Context, x float64, text *canvas.Text) {
	h := text.Bounds().H
	c.DrawText(x, y, text)
	y -= h + 10.0
}

// func draw(c *canvas.Context, int zoom, int x, int y) {
// 	c.SetView(canvas.Identity.Scale(2, 2).Translate(-100, -100))
// 	c.SetFillColor(canvas.White)
// 	c.DrawPath(0, 0, canvas.Rectangle(200, 200))

// 	c.SetFillColor(canvas.Black)

// 	// headerFace := fontFamily.Face(28.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)
// 	// textFace := fontFamily.Face(12.0, canvas.Black, canvas.FontRegular, canvas.FontNormal)

// 	// drawText(c, 30.0, canvas.NewTextBox(headerFace, "Document Example", 0.0, 0.0, canvas.Left, canvas.Top, 0.0, 0.0))
// 	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[0], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))

// 	// p := &canvas.Path{}

// 	// p.MoveTo(100, 100)
// 	// p := canvas.Circle(50)

// 	c.DrawPath(100, 100, canvas.Circle(50))

// 	// lenna, err := os.Open("../lenna.png")
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// img, err := png.Decode(lenna)
// 	// if err != nil {
// 	// 	panic(err)
// 	// }
// 	// imgDPM := 15.0
// 	// imgWidth := float64(img.Bounds().Max.X) / imgDPM
// 	// imgHeight := float64(img.Bounds().Max.Y) / imgDPM
// 	// c.DrawImage(170.0-imgWidth, y-imgHeight, img, imgDPM)

// 	// imgWidth := 50.
// 	// imgHeight := 50.

// 	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[1], 140.0-imgWidth-10.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
// 	// drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[2], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
// 	//drawText(c, 30.0, canvas.NewTextBox(textFace, lorem[3], 140.0, 0.0, canvas.Justify, canvas.Top, 5.0, 0.0))
// }
