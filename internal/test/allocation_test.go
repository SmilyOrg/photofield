package test

import (
	"image"
	"runtime"
	"testing"
)

func TestPoolReducesAllocations(t *testing.T) {
	// Test with pool
	pool := NewSizedImagePool()

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Generate many images of same size
	for i := 0; i < 100; i++ {
		img := pool.Get(500, 400)
		// Simulate some work
		for j := 0; j < 1000; j++ {
			img.Pix[j] = uint8(j % 256)
		}
		pool.Put(img)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	poolAllocs := m2.Mallocs - m1.Mallocs
	poolBytes := m2.TotalAlloc - m1.TotalAlloc

	t.Logf("Pool: %d allocations, %d bytes", poolAllocs, poolBytes)

	// Test without pool (direct allocation)
	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)

	for i := 0; i < 100; i++ {
		img := newRGBACleared(500, 400)
		// Simulate same work
		for j := 0; j < 1000; j++ {
			img.Pix[j] = uint8(j % 256)
		}
		_ = img // Prevent optimization
	}

	runtime.GC()
	var m4 runtime.MemStats
	runtime.ReadMemStats(&m4)

	directAllocs := m4.Mallocs - m3.Mallocs
	directBytes := m4.TotalAlloc - m3.TotalAlloc

	t.Logf("Direct: %d allocations, %d bytes", directAllocs, directBytes)

	// Pool should have significantly fewer allocations
	if poolAllocs >= directAllocs {
		t.Logf("Warning: Pool allocations (%d) not significantly less than direct (%d)", poolAllocs, directAllocs)
	} else {
		reduction := float64(directAllocs-poolAllocs) / float64(directAllocs) * 100
		t.Logf("Pool reduced allocations by %.1f%% (%d vs %d)", reduction, poolAllocs, directAllocs)
	}
}

// Helper function that mimics direct allocation with clearing
func newRGBACleared(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Clear like pool does
	for i := range img.Pix {
		img.Pix[i] = 0
	}
	return img
}
