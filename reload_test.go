package main

import (
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
)

func TestReloadLeaks(t *testing.T) {
	n := 10
	maxObjectsDiff := int64(1000)
	dataDir := "./data"
	initDefaults()
	appConfig, err := loadConfig(dataDir)
	if err != nil {
		log.Printf("unable to load configuration: %v", err)
		return
	}

	applyConfig(appConfig)

	var memStats runtime.MemStats

	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&memStats)
	initialObjects := memStats.HeapObjects
	initialSize := memStats.HeapAlloc

	// Capture heap profile before reloading
	beforeProfile, err := os.Create("profiles/reload-before.pprof")
	if err != nil {
		log.Printf("unable to create heap profile: %v", err)
		return
	}
	pprof.WriteHeapProfile(beforeProfile)
	beforeProfile.Close()

	for i := 0; i < n; i++ {
		applyConfig(appConfig)

		runtime.GC()
		runtime.ReadMemStats(&memStats)
		objectsDiff := int64(memStats.HeapObjects) - int64(initialObjects)
		sizeDiff := int64(memStats.HeapAlloc) - int64(initialSize)
		log.Printf("after %v reloads, %v objects leaked, %.2f per reload, %v bytes leaked, %.2f per reload", i+1, objectsDiff, float64(objectsDiff)/float64(i+1), sizeDiff, float64(sizeDiff)/float64(i+1))
	}

	runtime.ReadMemStats(&memStats)
	objectsDiff := int64(memStats.HeapObjects) - int64(initialObjects)
	if objectsDiff > maxObjectsDiff {
		t.Errorf("after %v reloads, %v objects leaked, %.2f per reload", n, objectsDiff, float64(objectsDiff)/float64(n))
	}

	// Capture heap profile after reloading
	afterProfile, err := os.Create("profiles/reload-after.pprof")
	if err != nil {
		log.Printf("unable to create heap profile: %v", err)
		return
	}
	pprof.WriteHeapProfile(afterProfile)
	afterProfile.Close()

}

func BenchmarkReload(b *testing.B) {
	dataDir := "./data"
	initDefaults()
	appConfig, err := loadConfig(dataDir)
	if err != nil {
		log.Printf("unable to load configuration: %v", err)
		return
	}

	for i := 0; i < b.N; i++ {
		applyConfig(appConfig)
	}
}
