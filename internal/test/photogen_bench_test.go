package test

import (
	"image"
	"math/rand"
	"os"
	"testing"
)

// BenchmarkImageGeneration benchmarks the core image generation pipeline
func BenchmarkImageGeneration(b *testing.B) {
	g := &TestImageGenerator{
		baseDir: "testdata",
		generators: []generator{
			generateCheckered,
			generateStripes,
		},
	}

	spec := ImageSpec{
		Width:  100,
		Height: 75,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := rand.New(rand.NewSource(int64(i)))

		// Create image
		img := image.NewRGBA(image.Rect(0, 0, spec.Width, spec.Height))

		// Generate pattern
		generatorIndex := r.Intn(len(g.generators))
		g.generators[generatorIndex](img, r)

		// Draw text
		g.drawText(img, "test_image")
	}
}

// BenchmarkJPEGEncoding benchmarks just the JPEG encoding step
func BenchmarkJPEGEncoding(b *testing.B) {
	g := &TestImageGenerator{}

	// Pre-generate an image
	img := image.NewRGBA(image.Rect(0, 0, 100, 75))
	r := rand.New(rand.NewSource(42))
	generateCheckered(img, r)
	g.drawText(img, "benchmark")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Benchmark JPEG encoding to /dev/null equivalent
		if err := g.saveJPEG(img, "/tmp/bench_test.jpg"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGenerateCheckered benchmarks the checkered pattern generator
func BenchmarkGenerateCheckered(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 150))
	r := rand.New(rand.NewSource(42))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		generateCheckered(img, r)
	}
}

// BenchmarkGenerateStripes benchmarks the stripes pattern generator
func BenchmarkGenerateStripes(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 150))
	r := rand.New(rand.NewSource(42))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		generateStripes(img, r)
	}
}

// BenchmarkDrawText benchmarks the text drawing function
func BenchmarkDrawText(b *testing.B) {
	g := &TestImageGenerator{}
	img := image.NewRGBA(image.Rect(0, 0, 200, 150))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		g.drawText(img, "benchmark_text")
	}
}

// BenchmarkCompleteImageGeneration benchmarks the complete pipeline including file I/O
func BenchmarkCompleteImageGeneration(b *testing.B) {
	g := &TestImageGenerator{
		baseDir: "/tmp",
		generators: []generator{
			generateCheckered,
			generateStripes,
		},
	}

	spec := ImageSpec{
		Width:  100,
		Height: 75,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r := rand.New(rand.NewSource(int64(i)))

		// Create image
		img := image.NewRGBA(image.Rect(0, 0, spec.Width, spec.Height))

		// Generate pattern
		generatorIndex := r.Intn(len(g.generators))
		g.generators[generatorIndex](img, r)

		// Draw text
		g.drawText(img, "bench_test")

		// Save to file
		filename := "/tmp/bench_test_" + string(rune('0'+i%10)) + ".jpg"
		if err := g.saveJPEG(img, filename); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelImageGeneration benchmarks parallel image generation
func BenchmarkParallelImageGeneration(b *testing.B) {
	g := &TestImageGenerator{
		baseDir: "/tmp",
		generators: []generator{
			generateCheckered,
			generateStripes,
		},
	}

	spec := ImageSpec{
		Width:  100,
		Height: 75,
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			r := rand.New(rand.NewSource(int64(i)))

			// Create image
			img := image.NewRGBA(image.Rect(0, 0, spec.Width, spec.Height))

			// Generate pattern
			generatorIndex := r.Intn(len(g.generators))
			g.generators[generatorIndex](img, r)

			// Draw text
			g.drawText(img, "parallel_bench")

			i++
		}
	})
}

// BenchmarkImageSizes benchmarks different image sizes
func BenchmarkImageSizes(b *testing.B) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"Small_100x75", 100, 75},
		{"Medium_200x150", 200, 150},
		{"Large_400x300", 400, 300},
		{"XLarge_800x600", 800, 600},
	}

	g := &TestImageGenerator{
		generators: []generator{generateCheckered},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				r := rand.New(rand.NewSource(int64(i)))
				img := image.NewRGBA(image.Rect(0, 0, size.width, size.height))
				generateCheckered(img, r)
				g.drawText(img, "size_test")
			}
		})
	}
}

// BenchmarkGenerateDataset benchmarks the complete GenerateDataset function with cleanup
func BenchmarkGenerateDataset(b *testing.B) {
	// Create a small test dataset
	dataset := TestDataset{
		Name: "bench-dataset",
		Images: []ImageSpec{
			{Width: 100, Height: 75},
			{Width: 150, Height: 100},
		},
		Samples: 2, // 2 samples per spec = 4 total images
		Seed:    42,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Create temp directory for this run
		tempDir, err := os.MkdirTemp("", "photogen_bench_")
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		// Run the complete dataset generation
		images, err := GenerateTestDataset(tempDir, dataset)
		if err != nil {
			b.Fatal(err)
		}

		if len(images) != 4 {
			b.Fatalf("Expected 4 images, got %d", len(images))
		}

		b.StopTimer()

		// Clean up files between runs
		err = os.RemoveAll(tempDir)
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
	}
}

// BenchmarkGenerateDatasetLarge benchmarks with more images to simulate CI workload
func BenchmarkGenerateDatasetLarge(b *testing.B) {
	// Simulate a CI-like workload with 100 images
	dataset := TestDataset{
		Name: "bench-large-dataset",
		Images: []ImageSpec{
			{Width: 100, Height: 75},
			{Width: 150, Height: 100},
			{Width: 200, Height: 75},
			{Width: 100, Height: 100},
			{Width: 150, Height: 75},
		},
		Samples: 20, // 5 specs × 20 samples = 100 total images
		Seed:    42,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Create temp directory for this run
		tempDir, err := os.MkdirTemp("", "photogen_bench_large_")
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		// Run the complete dataset generation
		images, err := GenerateTestDataset(tempDir, dataset)
		if err != nil {
			b.Fatal(err)
		}

		if len(images) != 100 {
			b.Fatalf("Expected 100 images, got %d", len(images))
		}

		b.StopTimer()

		// Clean up files between runs
		err = os.RemoveAll(tempDir)
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
	}
}

// BenchmarkGenerateDatasetSmall benchmarks with fewer images to simulate a lightweight workload
func BenchmarkGenerateDatasetManySmall(b *testing.B) {
	// Simulate a CI-like workload with 300 images
	dataset := TestDataset{
		Name: "bench-manysmall-dataset",
		Images: []ImageSpec{
			{Width: 30, Height: 20},
		},
		Samples: 300,
		Seed:    42,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()

		// Create temp directory for this run
		tempDir, err := os.MkdirTemp("", "photogen_bench_large_")
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()

		// Run the complete dataset generation
		images, err := GenerateTestDataset(tempDir, dataset)
		if err != nil {
			b.Fatal(err)
		}

		if len(images) != 300 {
			b.Fatalf("Expected 300 images, got %d", len(images))
		}

		b.StopTimer()

		// Clean up files between runs
		err = os.RemoveAll(tempDir)
		if err != nil {
			b.Fatal(err)
		}

		b.StartTimer()
	}
}
