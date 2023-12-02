package main

import (
	"context"
	"embed"
	"fmt"
	gio "io"
	"math/rand"
	"os"
	"path"
	"photofield/io"
	"photofield/io/ffmpeg"
	"photofield/io/goexif"
	"photofield/io/goimage"
	"photofield/io/ristretto"
	"photofield/io/sqlite"
	"photofield/io/thumb"
	"testing"
	"time"

	"zombiezen.com/go/sqlite/sqlitex"
)

var dir = "photos/"

// var dir = "E:/photos/"

var files = []struct {
	name string
	path string
}{
	{name: "P1110220", path: "test/P1110220.JPG"},

	// {name: "logo", path: "formats/logo.png"},
	// {name: "P1110220", path: "formats/P1110220.jpg"},
	// {name: "palette", path: "formats/i_palettes01_04.png"},
	// {name: "cow", path: "formats/cow.avif"},
}

func createTestSources() io.Sources {
	var goimg goimage.Image
	var ffmpegPath = ffmpeg.FindPath()
	return io.Sources{
		// cache,
		sqlite.New(path.Join(dir, "../data/photofield.thumbs.db"), embed.FS{}),
		goexif.Exif{},
		thumb.New(
			"S",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_S.jpg",
			io.FitInside,
			120,
			120,
		),
		thumb.New(
			"SM",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_SM.jpg",
			io.FitOutside,
			240,
			240,
		),
		thumb.New(
			"M",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_M.jpg",
			io.FitOutside,
			320,
			320,
		),
		thumb.New(
			"B",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_B.jpg",
			io.FitInside,
			640,
			640,
		),
		thumb.New(
			"XL",
			"{{.Dir}}@eaDir/{{.Filename}}/SYNOPHOTO_THUMB_XL.jpg",
			io.FitOutside,
			1280,
			1280,
		),
		goimg,
		ffmpeg.FFmpeg{
			Path:   ffmpegPath,
			Width:  128,
			Height: 128,
			Fit:    io.FitInside,
		},
		ffmpeg.FFmpeg{
			Path:   ffmpegPath,
			Width:  256,
			Height: 256,
			Fit:    io.FitInside,
		},
		ffmpeg.FFmpeg{
			Path:   ffmpegPath,
			Width:  512,
			Height: 512,
			Fit:    io.FitInside,
		},
		ffmpeg.FFmpeg{
			Path:   ffmpegPath,
			Width:  1280,
			Height: 1280,
			Fit:    io.FitInside,
		},
		ffmpeg.FFmpeg{
			Path:   ffmpegPath,
			Width:  4096,
			Height: 4096,
			Fit:    io.FitInside,
		},
	}
}

func BenchmarkSources(b *testing.B) {
	var cache = ristretto.New()
	var goimg goimage.Image
	sources := createTestSources()
	ctx := context.Background()
	for _, bm := range files {
		bm := bm
		p := path.Join(dir, bm.path)
		id := io.ImageId(1)
		r := goimg.Get(ctx, id, p)
		for i := 0; i < 100; i++ {
			cache.Set(ctx, id, p, r)
			time.Sleep(1 * time.Millisecond)
			r = cache.Get(ctx, id, p)
			if r.Image != nil || r.Error != nil {
				break
			}
		}

		b.Run(bm.name, func(b *testing.B) {
			for _, l := range sources {
				r := l.Get(ctx, id, p)
				img := r.Image
				err := r.Error
				if err != nil {
					b.Error(err)
				}
				if img == nil {
					b.Errorf("image not found: %d %s", id, p)
					continue
				} else {
					b.Logf("size: %d x %d", img.Bounds().Dx(), img.Bounds().Dy())
				}

				// b.ReportMetric(float64(img.Bounds().Dx()), "px")

				b.Run(fmt.Sprintf("%s-%dx%d", l.Name(), img.Bounds().Dx(), img.Bounds().Dy()), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						r := l.Get(ctx, id, p)
						if r.Error != nil {
							b.Error(r.Error)
						}
					}
				})
			}
		})
	}
}

func TestCost(t *testing.T) {
	sources := createTestSources()
	cases := []struct {
		zoom int
		o    io.Size
		size io.Size
		name string
	}{
		// {zoom: -1, size: io.Size{X: 1, Y: 1}, name: "bla"},
		{zoom: 0, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 120, Y: 80}, name: "thumb-120x120-S"},
		{zoom: 1, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 240, Y: 160}, name: "sqlite"},
		{zoom: 2, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 480, Y: 320}, name: "thumb-320x320-M"},
		{zoom: 3, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 960, Y: 640}, name: "thumb-1280x1280-XL"},
		{zoom: 4, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 1920, Y: 1280}, name: "thumb-1280x1280-XL"},
		{zoom: 5, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 3840, Y: 2560}, name: "ffmpeg-4096x4096-in"},
		{zoom: 6, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 7680, Y: 5120}, name: "image"},
		{zoom: 7, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 15360, Y: 10240}, name: "image"},
		{zoom: 8, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 30720, Y: 20480}, name: "image"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%dx%d-%d", c.o.X, c.o.Y, c.zoom), func(t *testing.T) {
			costs := sources.EstimateCost(c.o, c.size)
			costs.Sort()
			for i, c := range costs {
				t.Logf("%4d %6f %s\n", i, c.Cost, c.Name())
			}
			if costs[0].Name() != c.name {
				t.Errorf("unexpected smallest source %s", costs[0].Name())
			}
		})
	}
}

func TestCostSmallest(t *testing.T) {
	sources := createTestSources()
	costs := sources.EstimateCost(io.Size{X: 5472, Y: 3648}, io.Size{X: 1, Y: 1})
	costs.Sort()
	for i, c := range costs {
		fmt.Printf("%4d %6f %s\n", i, c.Cost, c.Name())
	}
	if costs[0].Name() != "thumb-120x120-S" {
		t.Errorf("unexpected smallest source %s", costs[0].Name())
	}
}

func BenchmarkSqlite(b *testing.B) {

	pool, err := sqlitex.Open(path.Join(dir, "test/photofield.thumbs.db"), 0, 10)
	if err != nil {
		panic(err)
	}
	c := pool.Get(context.Background())
	defer pool.Put(c)

	stmt := c.Prep(`
		SELECT data
		FROM thumb256
		WHERE id == ?;`)
	defer stmt.Reset()

	maxid := int64(1000000)

	for i := 0; i < b.N; i++ {
		id := 1 + rand.Int63n(maxid)
		stmt.BindInt64(1, id)
		exists, err := stmt.Step()
		if err != nil {
			b.Error(err)
		}
		if !exists {
			b.Errorf("id not found: %d", id)
		}
		r := stmt.ColumnReader(1)
		gio.ReadAll(r)
		stmt.Reset()
	}
}

func BenchmarkFile(b *testing.B) {

	maxid := int64(1000000)

	for i := 0; i < b.N; i++ {
		id := 1 + rand.Int63n(maxid)
		path := path.Join(dir, fmt.Sprintf("test/thumb/%d.jpg", id))
		_, err := os.ReadFile(path)
		if err != nil {
			b.Error(err)
		}
	}
}

// func BenchmarkThumbs(b *testing.B) {
// 	l := GoImage{}
// 	ctx := context.Background()
// 	for _, bc := range files {
// 		bc := bc // capture range variable
// 		b.
// 		// t.Run(tc.Name, func(t *testing.T) {
// 		//     t.Parallel()
// 		//     ...
// 		// })
// 	}
// 	// for i := 0; i < b.N; i++ {
// 	// 	// l.Load(ctx, "photos/formats/logo.png")
// 	// 	// l.Load(ctx, "P1110220.JPG")
// 	// 	// _, err := l.Load(ctx, "C:/w/photofield/photos/formats/carina-nebula-high-resolution_52259221868_o.png")
// 	// 	_, err := l.Load(ctx, "C:/w/photofield/photos/formats/logo.png")
// 	// 	// _, err := l.Load(ctx, "C:/w/photofield/photos/formats/P1110220.jpg")
// 	// 	if err != nil {
// 	// 		b.Error(err)
// 	// 	}
// 	// 	// time.Sleep(100 * time.Millisecond)
// 	// }
// }
