package test

import (
	"image"
	"testing"
)

func TestSizedImagePool(t *testing.T) {
	pool := NewSizedImagePool()

	// Test getting images of different sizes
	img1 := pool.Get(100, 200)
	img2 := pool.Get(100, 200) // Same size
	img3 := pool.Get(150, 150) // Different size

	// Verify correct dimensions
	if img1.Bounds().Dx() != 100 || img1.Bounds().Dy() != 200 {
		t.Errorf("img1 has wrong dimensions: got %dx%d, want 100x200", img1.Bounds().Dx(), img1.Bounds().Dy())
	}

	if img2.Bounds().Dx() != 100 || img2.Bounds().Dy() != 200 {
		t.Errorf("img2 has wrong dimensions: got %dx%d, want 100x200", img2.Bounds().Dx(), img2.Bounds().Dy())
	}

	if img3.Bounds().Dx() != 150 || img3.Bounds().Dy() != 150 {
		t.Errorf("img3 has wrong dimensions: got %dx%d, want 150x150", img3.Bounds().Dx(), img3.Bounds().Dy())
	}

	// Verify images are cleared (all pixels should be 0)
	for i := range img1.Pix {
		if img1.Pix[i] != 0 {
			t.Errorf("img1 pixel at index %d is not cleared: got %d, want 0", i, img1.Pix[i])
			break
		}
	}

	// Put images back
	pool.Put(img1)
	pool.Put(img2)
	pool.Put(img3)

	// Get the same size again - should reuse from pool
	img4 := pool.Get(100, 200)
	if img4.Bounds().Dx() != 100 || img4.Bounds().Dy() != 200 {
		t.Errorf("img4 has wrong dimensions: got %dx%d, want 100x200", img4.Bounds().Dx(), img4.Bounds().Dy())
	}

	// Should be cleared again
	for i := range img4.Pix {
		if img4.Pix[i] != 0 {
			t.Errorf("img4 pixel at index %d is not cleared: got %d, want 0", i, img4.Pix[i])
			break
		}
	}

	pool.Put(img4)
}

func TestSizedImagePoolConcurrency(t *testing.T) {
	pool := NewSizedImagePool()

	// Test concurrent access to the pool
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Each goroutine uses a different size to test concurrent pool creation
			width := 100 + id*10
			height := 100 + id*5

			for j := 0; j < 100; j++ {
				img := pool.Get(width, height)
				if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
					t.Errorf("goroutine %d: wrong dimensions: got %dx%d, want %dx%d",
						id, img.Bounds().Dx(), img.Bounds().Dy(), width, height)
				}
				pool.Put(img)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkSizedImagePool(b *testing.B) {
	pool := NewSizedImagePool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		img := pool.Get(1024, 768)
		pool.Put(img)
	}
}

func BenchmarkDirectAllocation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 1024, 768))
		// Clear the image like the pool does
		for i := range img.Pix {
			img.Pix[i] = 0
		}
		_ = img // Prevent optimization
	}
}
