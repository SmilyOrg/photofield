package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"photofield/internal/io"
	"photofield/internal/io/djpeg"
	"photofield/internal/io/exiftool"
	"photofield/internal/io/ffmpeg"
	"photofield/internal/io/goexif"
	"photofield/internal/io/goimage"
	"photofield/internal/io/ristretto"
	"photofield/internal/io/sqlite"
	"photofield/internal/io/thumb"
	"photofield/internal/test"
	"strings"
	"testing"
	"time"

	"golang.org/x/image/draw"
)

func createTestSources(t testing.TB) io.Sources {
	var goimg goimage.Image
	var ffmpegPath = ffmpeg.FindPath()
	dbPath := path.Join(t.TempDir(), "photofield.thumbs.db")
	return io.Sources{
		// cache,
		sqlite.New(dbPath),
		goexif.Exif{
			Width:  256,
			Height: 256,
			Fit:    io.FitInside,
		},
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

// prepareBenchmarkThumbnails generates thumbnails for all sources that need them
// This ensures the benchmark tests won't fail due to missing thumbnails
func prepareBenchmarkThumbnails(b *testing.B, sources io.Sources, images []test.GeneratedImage) error {
	ctx := context.Background()

	b.Logf("Preparing thumbnails for %d sources and %d images...", len(sources), len(images))

	for _, img := range images {
		id := io.ImageId(1)

		// Generate thumbnails using goimage source first (this always works)
		var goimg goimage.Image
		origResult := goimg.Get(ctx, id, img.Path)
		if origResult.Error != nil {
			return fmt.Errorf("failed to load original image %s: %w", img.Path, origResult.Error)
		}
		if origResult.Image == nil {
			return fmt.Errorf("no image data for %s", img.Path)
		}

		for _, source := range sources {
			switch s := source.(type) {
			case *sqlite.Source:
				// Generate thumbnail for SQLite storage
				err := prepareSQLiteThumbnail(ctx, s, id, img.Path, origResult.Image)
				if err != nil {
					b.Logf("Warning: failed to prepare SQLite thumbnail for %s: %v", img.Path, err)
				}

			case *thumb.Thumb:
				// Generate thumbnail files for Synology-style storage
				err := prepareSynologyThumbnail(ctx, s, id, img.Path, origResult.Image)
				if err != nil {
					b.Logf("Warning: failed to prepare Synology thumbnail for %s: %v", img.Path, err)
				}

			case goexif.Exif:
				// Embed EXIF thumbnail in the image file
				err := prepareExifThumbnail(ctx, id, img.Path, origResult.Image)
				if err != nil {
					b.Logf("Warning: failed to prepare EXIF thumbnail for %s: %v", img.Path, err)
				}
			}
		}
	}

	b.Logf("Thumbnail preparation completed")
	return nil
}

// resizeImage creates a thumbnail from the original image with proper scaling
func resizeImage(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()

	// Calculate thumbnail dimensions maintaining aspect ratio
	var desiredW, desiredH int
	if origW > origH {
		desiredW = maxWidth
		desiredH = (origH * maxWidth) / origW
	} else {
		desiredH = maxHeight
		desiredW = (origW * maxHeight) / origH
	}

	// Ensure minimum size
	if desiredW <= 0 || desiredH <= 0 {
		desiredW, desiredH = maxWidth, maxHeight
	}

	// Create the resized image using bilinear scaling
	resized := image.NewRGBA(image.Rect(0, 0, desiredW, desiredH))
	draw.ApproxBiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Src, nil)
	return resized
}

// prepareSQLiteThumbnail generates and stores a thumbnail in the SQLite database
func prepareSQLiteThumbnail(ctx context.Context, sqliteSource *sqlite.Source, id io.ImageId, imagePath string, origImage image.Image) error {
	// Create a 256x256 thumbnail by resizing the original image
	var buf bytes.Buffer
	thumbSize := 256

	// Resize the original image to create a realistic thumbnail
	thumbImg := resizeImage(origImage, thumbSize, thumbSize)

	// Encode as JPEG
	err := jpeg.Encode(&buf, thumbImg, &jpeg.Options{Quality: 90})
	if err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	// Store in SQLite database
	return sqliteSource.Write(uint32(id), buf.Bytes())
}

// prepareSynologyThumbnail generates and stores a thumbnail file in the @eaDir structure
func prepareSynologyThumbnail(ctx context.Context, thumbSource *thumb.Thumb, id io.ImageId, imagePath string, origImage image.Image) error {
	// Get the thumbnail path using the template
	dir := filepath.Dir(imagePath)
	filename := filepath.Base(imagePath)

	templateData := struct {
		Dir      string
		Filename string
	}{
		Dir:      dir + "/", // Add trailing slash to ensure proper path joining
		Filename: filename,  // Keep the full filename including extension
	}

	var pathBuf bytes.Buffer
	err := thumbSource.PathTemplate.Execute(&pathBuf, templateData)
	if err != nil {
		return fmt.Errorf("failed to execute path template: %w", err)
	}

	thumbPath := pathBuf.String()

	// Create directory structure
	thumbDir := filepath.Dir(thumbPath)
	err = os.MkdirAll(thumbDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail directory %s: %w", thumbDir, err)
	}

	// Create thumbnail image matching the thumb source dimensions
	bounds := origImage.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()

	// Apply the fit mode from the thumb source to calculate final size
	var thumbW, thumbH int
	maxW, maxH := thumbSource.Width, thumbSource.Height

	switch thumbSource.Fit {
	case io.FitInside:
		// Scale to fit inside the bounds
		if origW*maxH > origH*maxW {
			thumbW = maxW
			thumbH = (origH * maxW) / origW
		} else {
			thumbH = maxH
			thumbW = (origW * maxH) / origH
		}
	case io.FitOutside:
		// Scale to fill the bounds
		if origW*maxH < origH*maxW {
			thumbW = maxW
			thumbH = (origH * maxW) / origW
		} else {
			thumbH = maxH
			thumbW = (origW * maxH) / origH
		}
	default:
		thumbW, thumbH = maxW, maxH
	}

	if thumbW <= 0 || thumbH <= 0 {
		thumbW, thumbH = maxW, maxH
	}

	// Resize the original image to create a realistic thumbnail
	thumbImg := resizeImage(origImage, thumbW, thumbH)

	// Save thumbnail file
	file, err := os.Create(thumbPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file %s: %w", thumbPath, err)
	}
	defer file.Close()

	// Encode based on file extension
	ext := strings.ToLower(filepath.Ext(thumbPath))
	switch ext {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(file, thumbImg, &jpeg.Options{Quality: 90})
	case ".png":
		err = png.Encode(file, thumbImg)
	default:
		err = jpeg.Encode(file, thumbImg, &jpeg.Options{Quality: 90})
	}

	if err != nil {
		return fmt.Errorf("failed to encode thumbnail to %s: %w", thumbPath, err)
	}

	return nil
}

// prepareExifThumbnail embeds a JPEG thumbnail in the EXIF data of an image file
func prepareExifThumbnail(ctx context.Context, id io.ImageId, imagePath string, origImage image.Image) error {
	// Create a small thumbnail for EXIF embedding (typically 160x120 or similar)
	thumbSize := 160

	// Resize the original image to create a realistic thumbnail
	thumbImg := resizeImage(origImage, thumbSize, thumbSize)

	// Encode thumbnail as JPEG
	var thumbBuf bytes.Buffer
	err := jpeg.Encode(&thumbBuf, thumbImg, &jpeg.Options{Quality: 80})
	if err != nil {
		return fmt.Errorf("failed to encode EXIF thumbnail: %w", err)
	}

	// Use exiftool to embed the thumbnail
	// Create a temporary file for the thumbnail
	tmpDir := filepath.Dir(imagePath)
	// Use a hash based on image ID to avoid conflicts
	hash := uint32(id)
	thumbFile := filepath.Join(tmpDir, fmt.Sprintf(".thumb_%d.jpg", hash))
	defer os.Remove(thumbFile) // Clean up

	err = os.WriteFile(thumbFile, thumbBuf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write temporary thumbnail: %w", err)
	}

	// Find exiftool path
	exiftoolPath := exiftool.FindPath()
	if exiftoolPath == "" {
		return fmt.Errorf("exiftool not found in system PATH")
	}

	// Use exiftool to embed the thumbnail in the original image (call directly)
	cmd := exec.Command(exiftoolPath,
		"-overwrite_original",
		fmt.Sprintf("-ThumbnailImage<=%s", thumbFile),
		imagePath)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to embed EXIF thumbnail using exiftool: %w", err)
	}

	return nil
}

func BenchmarkSources(b *testing.B) {
	var cache = ristretto.New(1024 * 1024 * 1024)
	var goimg goimage.Image
	sources := createTestSources(b)
	ctx := context.Background()

	dataset := test.TestDataset{
		Name:    "benchmark",
		Seed:    12345,
		Samples: 1,
		Images: []test.ImageSpec{
			{Width: 5472, Height: 3648}, // P1110220 equivalent
			{Width: 4032, Height: 3024}, // Common camera resolution
			{Width: 1920, Height: 1080}, // HD resolution
		},
	}
	images, err := test.GenerateTestDataset("testdata", dataset)
	if err != nil {
		b.Fatalf("failed to generate benchmark dataset: %v", err)
	}

	// Postprocess: generate thumbnails for all sources to fix benchmark failures
	err = prepareBenchmarkThumbnails(b, sources, images)
	if err != nil {
		b.Fatalf("failed to prepare thumbnails: %v", err)
	}

	for _, img := range images {
		id := io.ImageId(1)
		r := goimg.Get(ctx, id, img.Path)
		for i := 0; i < 100; i++ {
			cache.Set(ctx, id, img.Path, r)
			time.Sleep(1 * time.Millisecond)
			r = cache.Get(ctx, id, img.Path)
			if r.Image != nil || r.Error != nil {
				break
			}
		}

		b.Run(img.Name, func(b *testing.B) {
			for _, l := range sources {
				r := l.Get(ctx, id, img.Path)
				image := r.Image
				err := r.Error

				if err != nil {
					b.Error(err)
				}
				if image == nil {
					b.Errorf("image not found: %d %s", id, img.Path)
					continue
				} else {
					b.Logf("size: %d x %d", image.Bounds().Dx(), image.Bounds().Dy())
				}

				// b.ReportMetric(float64(image.Bounds().Dx()), "px")

				b.Run(fmt.Sprintf("%s-%dx%d", l.Name(), image.Bounds().Dx(), image.Bounds().Dy()), func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						r := l.Get(ctx, id, img.Path)
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
	sources := createTestSources(t)
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
		{zoom: 6, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 7680, Y: 5120}, name: "original"},
		{zoom: 7, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 15360, Y: 10240}, name: "original"},
		{zoom: 8, o: io.Size{X: 5472, Y: 3648}, size: io.Size{X: 30720, Y: 20480}, name: "original"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%dx%d-%d", c.o.X, c.o.Y, c.zoom), func(t *testing.T) {
			costs := sources.EstimateCost(c.o, c.size)
			costs.Sort()
			for i, c := range costs {
				t.Logf("%4d %6.0f %s", i, c.Cost, c.Name())
			}
			if costs[0].Name() != c.name {
				t.Errorf("unexpected best source %s", costs[0].Name())
			}
		})
	}
}

func TestCostSmallest(t *testing.T) {
	sources := createTestSources(t)
	costs := sources.EstimateCost(io.Size{X: 5472, Y: 3648}, io.Size{X: 1, Y: 1})
	costs.Sort()
	for i, c := range costs {
		fmt.Printf("%4d %6f %s\n", i, c.Cost, c.Name())
	}
	if costs[0].Name() != "thumb-120x120-S" {
		t.Errorf("unexpected smallest source %s", costs[0].Name())
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
		Samples: 3,
		Images: []test.ImageSpec{
			{Width: 5472, Height: 3648},
			{Width: 1920, Height: 1080},
			{Width: 640, Height: 480},
			{Width: 256, Height: 256},
			{Width: 200, Height: 100},
			{Width: 32, Height: 32},
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
