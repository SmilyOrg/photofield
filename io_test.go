package main

import (
	"context"
	"embed"
	"fmt"
	gio "io"
	"math/rand"
	"os"
	"path"
	"photofield/internal/test"
	"photofield/io"
	"photofield/io/djpeg"
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
	{name: "P1110220", path: "Test/2023-02-03 09.25.08.jpg"},

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
	var cache = ristretto.New(1024 * 1024 * 1024)
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

// Convert width, height, and max side length to a thumbnail size
func fitToSize(width, height, maxSide int) (int, int) {
	if width <= 0 || height <= 0 || maxSide <= 0 {
		return 0, 0
	}

	if width > height {
		if width > maxSide {
			height = (height * maxSide) / width
			width = maxSide
		}
	} else {
		if height > maxSide {
			width = (width * maxSide) / height
			height = maxSide
		}
	}
	return width, height
}

func TestThumbnailGeneration(t *testing.T) {
	dataset := test.TestDataset{
		Name:    "thumbs",
		Seed:    12345,
		Samples: 10,
		Images: []test.ImageSpec{
			{Width: 5472, Height: 3648},
			{Width: 4032, Height: 3024},
			{Width: 1920, Height: 1080},
			{Width: 1280, Height: 720},
			{Width: 800, Height: 600},
			{Width: 640, Height: 480},
			{Width: 320, Height: 240},
			{Width: 256, Height: 256},
			{Width: 200, Height: 100},
			{Width: 64, Height: 64},
			{Width: 32, Height: 32},
			{Width: 3648, Height: 5472},
			{Width: 3024, Height: 4032},
			{Width: 1080, Height: 1920},
			{Width: 240, Height: 320},
		},
	}
	images, err := test.GenerateTestDataset("testdata", dataset)
	if err != nil {
		t.Fatalf("failed to generate test dataset: %v", err)
	}
	thumbSize := 256
	ffmpegPath := ffmpeg.FindPath()
	djpegPath := djpeg.FindPath()
	sources := io.Sources{
		goimage.Image{Width: thumbSize, Height: thumbSize},
		ffmpeg.FFmpeg{Width: thumbSize, Height: thumbSize, Path: ffmpegPath, Fit: io.FitInside},
		djpeg.Djpeg{Width: thumbSize, Height: thumbSize, Path: djpegPath},
	}
	for _, src := range sources {
		t.Run(src.Name(), func(t *testing.T) {
			for _, img := range images {
				img := img // capture loop variable
				t.Run(img.Name, func(t *testing.T) {
					t.Parallel()
					ctx := context.Background()
					id := io.ImageId(0)
					r := src.Get(ctx, id, img.Path)
					if r.Error != nil {
						t.Errorf("failed to load image %s: %v", img.Path, r.Error)
						return
					}
					if r.Image == nil {
						t.Errorf("image not found: %s", img.Path)
						return
					}
					w, h := r.Image.Bounds().Dx(), r.Image.Bounds().Dy()
					// fmt.Printf("Image %s: %dx%d\n", img.Path, w, h)
					desiredW, desiredH := fitToSize(w, h, thumbSize)
					if img.Spec.Width <= thumbSize && img.Spec.Height <= thumbSize {
						desiredW, desiredH = img.Spec.Width, img.Spec.Height
					}
					if w != desiredW || h != desiredH {
						t.Errorf("unexpected thumbnail size for %s: got %dx%d, want %dx%d", img.Path, w, h, desiredW, desiredH)
						return
					}
				})
			}
		})
	}
}

// The thumbnails should be generated with the correct baked-in orientation
// based on the EXIF orientation tag.
func TestThumbnailGenerationOrientation(t *testing.T) {
	dataset := test.TestDataset{
		Name:    "thumbs-orientation",
		Seed:    12345,
		Samples: 1,
		Images: []test.ImageSpec{
			{Width: 60, Height: 30}, // No tag
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Horizontal (normal)"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Mirror horizontal"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Rotate 180"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Mirror vertical"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Mirror horizontal and rotate 270 CW"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Rotate 90 CW"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Mirror horizontal and rotate 90 CW"}},
			{Width: 60, Height: 30, ExifTags: map[string]string{"Orientation": "Rotate 270 CW"}},
		},
	}
	images, err := test.GenerateTestDataset("testdata", dataset)
	if err != nil {
		t.Fatalf("failed to generate test dataset: %v", err)
	}
	thumbSize := 256
	ffmpegPath := ffmpeg.FindPath()
	djpegPath := djpeg.FindPath()
	sources := io.Sources{
		goimage.Image{Width: thumbSize, Height: thumbSize},
		ffmpeg.FFmpeg{Width: thumbSize, Height: thumbSize, Path: ffmpegPath, Fit: io.FitInside},
		djpeg.Djpeg{Width: thumbSize, Height: thumbSize, Path: djpegPath},
	}
	for _, src := range sources {
		t.Run(src.Name(), func(t *testing.T) {
			for _, img := range images {
				img := img // capture loop variable
				t.Run(img.Name, func(t *testing.T) {
					t.Parallel()
					ctx := context.Background()
					id := io.ImageId(0)
					r := src.Get(ctx, id, img.Path)
					if r.Error != nil {
						t.Errorf("failed to load image %s: %v", img.Path, r.Error)
						return
					}
					if r.Image == nil {
						t.Errorf("image not found: %s", img.Path)
						return
					}
					// Check that the image has the correct orientation
					// or that the returned orientation is set to SourceInfoOrientation
					if r.Orientation == io.Normal || r.Orientation == io.SourceInfoOrientation {
						// Load the original image to compare orientation
						origSrc := goimage.Image{}
						origR := origSrc.Get(context.Background(), id, img.Path)
						if origR.Error != nil {
							t.Errorf("failed to load original image %s: %v", img.Path, origR.Error)
							return
						}

						// Check if dimensions match expected orientation
						thumbW, thumbH := r.Image.Bounds().Dx(), r.Image.Bounds().Dy()
						origW, origH := origR.Image.Bounds().Dx(), origR.Image.Bounds().Dy()

						// Sample a few pixels to verify the transformation was applied
						origBounds := origR.Image.Bounds()
						thumbBounds := r.Image.Bounds()

						// Check corner pixels to verify orientation
						origTopLeft := origR.Image.At(origBounds.Min.X, origBounds.Min.Y)
						origTopRight := origR.Image.At(origBounds.Max.X-1, origBounds.Min.Y)
						origBottomLeft := origR.Image.At(origBounds.Min.X, origBounds.Max.Y-1)
						origBottomRight := origR.Image.At(origBounds.Max.X-1, origBounds.Max.Y-1)

						thumbTopLeft := r.Image.At(thumbBounds.Min.X, thumbBounds.Min.Y)
						thumbTopRight := r.Image.At(thumbBounds.Max.X-1, thumbBounds.Min.Y)
						thumbBottomLeft := r.Image.At(thumbBounds.Min.X, thumbBounds.Max.Y-1)
						thumbBottomRight := r.Image.At(thumbBounds.Max.X-1, thumbBounds.Max.Y-1)

						// For rotated images (90/270 degrees), dimensions should be swapped
						orientation := img.Spec.ExifTags["Orientation"]

						tl, tr, bl, br := origTopLeft, origTopRight, origBottomLeft, origBottomRight
						if r.Orientation == io.Normal {
							shouldBeRotated := orientation == "Rotate 90 CW" || orientation == "Rotate 270 CW" ||
								orientation == "Mirror horizontal and rotate 270 CW" || orientation == "Mirror horizontal and rotate 90 CW"

							if shouldBeRotated {
								if (thumbW > thumbH) == (origW > origH) {
									t.Errorf("image %s appears not to be rotated: thumb=%dx%d, orig=%dx%d, orientation=%s",
										img.Path, thumbW, thumbH, origW, origH, orientation)
								}
							} else {
								if (thumbW > thumbH) != (origW > origH) {
									t.Errorf("image %s appears incorrectly rotated: thumb=%dx%d, orig=%dx%d, orientation=%s",
										img.Path, thumbW, thumbH, origW, origH, orientation)
								}
							}

							tl, tr, bl, br = test.TransformCornersByName(
								origTopLeft, origTopRight, origBottomLeft, origBottomRight,
								orientation,
							)
						}

						if !test.ColorsSimilar(thumbTopLeft, tl, 4) || !test.ColorsSimilar(thumbTopRight, tr, 4) ||
							!test.ColorsSimilar(thumbBottomLeft, bl, 4) || !test.ColorsSimilar(thumbBottomRight, br, 4) {
							t.Errorf("thumbnail %s has incorrect %s orientation", img.Path, orientation)
						}
					} else {
						t.Errorf("unexpected orientation for %s: got %d, want %d", img.Path, r.Orientation, io.SourceInfoOrientation)
						return
					}
				})
			}
		})
	}
}

func BenchmarkThumbs(b *testing.B) {
	dataset := test.TestDataset{
		Name:    "thumbs-bench",
		Seed:    12345,
		Samples: 1,
		Images: []test.ImageSpec{
			{Width: 5472, Height: 3648},
			{Width: 1920, Height: 1080},
			{Width: 640, Height: 480},
			{Width: 256, Height: 256},
			{Width: 64, Height: 64},
		},
	}
	images, err := test.GenerateTestDataset("testdata", dataset)
	if err != nil {
		b.Fatalf("failed to generate test dataset: %v", err)
	}
	thumbSize := 256
	ffmpegPath := ffmpeg.FindPath()
	djpegPath := djpeg.FindPath()
	sources := io.Sources{
		goimage.Image{Width: thumbSize, Height: thumbSize},
		ffmpeg.FFmpeg{Width: thumbSize, Height: thumbSize, Path: ffmpegPath, Fit: io.FitInside},
		djpeg.Djpeg{Width: thumbSize, Height: thumbSize, Path: djpegPath},
	}
	ctx := context.Background()
	for _, src := range sources {
		b.Run(fmt.Sprintf("source=%s/fun=Get", src.Name()), func(b *testing.B) {
			for _, img := range images {
				id := io.ImageId(0)
				b.Run(fmt.Sprintf("width=%d/height=%d", img.Spec.Width, img.Spec.Height), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						r := src.Get(ctx, id, img.Path)
						if r.Error != nil {
							b.Errorf("failed to load image %s: %v", img.Path, r.Error)
						}
						if r.Image == nil {
							b.Errorf("image not found: %s", img.Path)
						}
					}
				})
			}
		})
		if srcs, ok := src.(io.GetterWithSize); ok {
			b.Run(fmt.Sprintf("source=%s/fun=GetWithSize", src.Name()), func(b *testing.B) {
				for _, img := range images {
					id := io.ImageId(0)
					b.Run(fmt.Sprintf("width=%d/height=%d", img.Spec.Width, img.Spec.Height), func(b *testing.B) {
						size := io.Size{X: img.Spec.Width, Y: img.Spec.Height}
						for i := 0; i < b.N; i++ {
							r := srcs.GetWithSize(ctx, id, img.Path, size)
							if r.Error != nil {
								b.Errorf("failed to load image %s: %v", img.Path, r.Error)
							}
							if r.Image == nil {
								b.Errorf("image not found: %s", img.Path)
							}
						}
					})
				}
			})
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
