package layout

import (
	"math/rand"
	"photofield/internal/image"
	"photofield/internal/render"
	"testing"

	"github.com/golang/geo/s2"
)

// generateClumpedGeoCoords generates realistic geo coordinates in clumps
// clumps: number of geographic clusters
// photosPerClump: number of photos in each cluster
// spreadKm: how spread out each clump is in kilometers
func generateClumpedGeoCoords(clumps, photosPerClump int, spreadKm float64, seed int64) []s2.LatLng {
	rnd := rand.New(rand.NewSource(seed))
	coords := make([]s2.LatLng, 0, clumps*photosPerClump)

	// Generate clump centers across the world
	// Using realistic ranges: lat [-60, 60], lng [-180, 180]
	clumpCenters := make([]s2.LatLng, clumps)
	for i := 0; i < clumps; i++ {
		lat := rnd.Float64()*120 - 60  // -60 to 60 degrees
		lng := rnd.Float64()*360 - 180 // -180 to 180 degrees
		clumpCenters[i] = s2.LatLngFromDegrees(lat, lng)
	}

	// Generate photos around each clump center
	for _, center := range clumpCenters {
		for j := 0; j < photosPerClump; j++ {
			// Convert spread from km to degrees (approximate)
			// 1 degree latitude ≈ 111 km
			spreadDeg := spreadKm / 111.0

			// Add random offset within the spread radius
			latOffset := (rnd.Float64()*2 - 1) * spreadDeg
			lngOffset := (rnd.Float64()*2 - 1) * spreadDeg

			lat := center.Lat.Degrees() + latOffset
			lng := center.Lng.Degrees() + lngOffset

			// Clamp to valid ranges
			if lat > 85 {
				lat = 85
			}
			if lat < -85 {
				lat = -85
			}
			if lng > 180 {
				lng -= 360
			}
			if lng < -180 {
				lng += 360
			}

			coords = append(coords, s2.LatLngFromDegrees(lat, lng))
		}
	}

	return coords
}

// generateMockInfos creates a channel of SourcedInfo with given coordinates
func generateMockInfos(coords []s2.LatLng) <-chan image.SourcedInfo {
	infos := make(chan image.SourcedInfo, len(coords))

	for i, coord := range coords {
		info := image.SourcedInfo{
			Id: image.ImageId(i),
			Info: image.Info{
				Width:  1920,
				Height: 1080,
				LatLng: coord,
			},
		}
		infos <- info
	}
	close(infos)

	return infos
}

// Benchmark scenarios
func BenchmarkLayoutMap_2PhotosAdjacent(b *testing.B) {
	coords := generateClumpedGeoCoords(1, 2, 0.01, 42) // 0.01km = 10m apart
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_10PhotosSmallClump(b *testing.B) {
	coords := generateClumpedGeoCoords(1, 10, 0.1, 42) // 100m spread
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_50PhotosSingleLocation(b *testing.B) {
	coords := generateClumpedGeoCoords(1, 50, 0.5, 42) // 500m spread
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_100PhotosSingleLocation(b *testing.B) {
	coords := generateClumpedGeoCoords(1, 100, 1.0, 42) // 1km spread
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_5Clumps_100Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(5, 100, 2.0, 42) // 500 total, 2km spread each
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_10Clumps_100Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 100, 2.0, 42) // 1000 total
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_20Clumps_100Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(20, 100, 2.0, 42) // 2000 total
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_50Clumps_100Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(50, 100, 5.0, 42) // 5000 total, 5km spread
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_100Clumps_100Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(100, 100, 5.0, 42) // 10000 total
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_10Clumps_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 1000, 10.0, 42) // 10000 total, 10km spread
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_100Clumps_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(100, 1000, 10.0, 42) // 100000 total
	benchmarkLayoutMap(b, coords)
}

// Spread variation benchmarks
func BenchmarkLayoutMap_VeryTight_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 100, 0.05, 42) // 50m spread - very tight
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_Tight_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 100, 0.5, 42) // 500m spread - tight
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_Medium_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 100, 5.0, 42) // 5km spread - medium
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_Wide_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 100, 50.0, 42) // 50km spread - wide
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_VeryWide_1000Photos(b *testing.B) {
	coords := generateClumpedGeoCoords(10, 100, 500.0, 42) // 500km spread - very wide
	benchmarkLayoutMap(b, coords)
}

// Edge cases
func BenchmarkLayoutMap_SinglePhoto(b *testing.B) {
	coords := generateClumpedGeoCoords(1, 1, 0, 42)
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_ManySmallClumps(b *testing.B) {
	coords := generateClumpedGeoCoords(1000, 10, 1.0, 42) // 10000 total, 1000 clumps
	benchmarkLayoutMap(b, coords)
}

func BenchmarkLayoutMap_FewLargeClumps(b *testing.B) {
	coords := generateClumpedGeoCoords(2, 5000, 20.0, 42) // 10000 total, 2 clumps
	benchmarkLayoutMap(b, coords)
}

// Helper function to run the actual benchmark
func benchmarkLayoutMap(b *testing.B, coords []s2.LatLng) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		infos := generateMockInfos(coords)
		scene := &render.Scene{}
		layout := Layout{
			ViewportWidth:  1920,
			ViewportHeight: 1080,
		}
		source := &image.Source{}
		b.StartTimer()

		LayoutMap(infos, layout, scene, source)
	}

	b.ReportMetric(float64(len(coords)), "photos")
}
