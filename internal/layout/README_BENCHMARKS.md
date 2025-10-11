# LayoutMap Benchmarks

This document describes the benchmarks available for testing the `LayoutMap` function performance.

## Overview

The `map_bench_test.go` file contains comprehensive benchmarks that test the map layout algorithm with realistic geographic coordinate distributions. The benchmarks simulate various real-world scenarios where photos are clustered geographically.

## Running Benchmarks

### Run all benchmarks
```bash
go test -bench=BenchmarkLayoutMap -benchmem ./internal/layout/
```

### Run specific benchmark
```bash
go test -bench=BenchmarkLayoutMap_10Clumps_100Photos -benchmem ./internal/layout/
```

### Run with limited iterations (faster testing)
```bash
go test -bench=BenchmarkLayoutMap -benchtime=1x ./internal/layout/
```

### Compare benchmarks with benchstat
```bash
# Run benchmarks and save results
go test -bench=BenchmarkLayoutMap -benchmem -count=5 ./internal/layout/ > old.txt

# After making changes
go test -bench=BenchmarkLayoutMap -benchmem -count=5 ./internal/layout/ > new.txt

# Compare
benchstat old.txt new.txt
```

## Benchmark Scenarios

### Small Scale Tests
- **SinglePhoto**: 1 photo - edge case baseline
- **2PhotosAdjacent**: 2 photos 10m apart
- **10PhotosSmallClump**: 10 photos in 100m radius
- **50PhotosSingleLocation**: 50 photos in 500m radius  
- **100PhotosSingleLocation**: 100 photos in 1km radius

### Medium Scale - Multiple Clumps
- **5Clumps_100Photos**: 500 total photos, 5 locations, 2km spread each
- **10Clumps_100Photos**: 1,000 photos, 10 locations, 2km spread each
- **20Clumps_100Photos**: 2,000 photos, 20 locations, 2km spread each
- **50Clumps_100Photos**: 5,000 photos, 50 locations, 5km spread each
- **100Clumps_100Photos**: 10,000 photos, 100 locations, 5km spread each

### Large Scale - Dense Clumps
- **10Clumps_1000Photos**: 10,000 photos, 10 dense clumps, 10km spread
- **100Clumps_1000Photos**: 100,000 photos, 100 clumps, 10km spread

### Spread Variation Tests (1000 photos)
Tests how different geographic spreads affect performance:
- **VeryTight_1000Photos**: 10 clumps, 50m spread (dense urban)
- **Tight_1000Photos**: 10 clumps, 500m spread (neighborhood)
- **Medium_1000Photos**: 10 clumps, 5km spread (city district)
- **Wide_1000Photos**: 10 clumps, 50km spread (metro area)
- **VeryWide_1000Photos**: 10 clumps, 500km spread (regional)

### Distribution Pattern Tests
- **ManySmallClumps**: 10,000 photos in 1,000 small clumps of 10
- **FewLargeClumps**: 10,000 photos in 2 large clumps of 5,000

## Understanding Results

The benchmarks report:
- **ns/op**: Nanoseconds per operation (lower is better)
- **photos**: Number of photos in the test (custom metric)
- **B/op**: Bytes allocated per operation (with -benchmem)
- **allocs/op**: Number of allocations per operation (with -benchmem)

Example output:
```
BenchmarkLayoutMap_10Clumps_100Photos-24    1    152847291 ns/op    1000 photos
```
This means the layout took ~153ms for 1,000 photos.

## Test Data Generation

The benchmarks use `generateClumpedGeoCoords()` which creates realistic coordinate distributions:
- **Clumps**: Number of geographic clusters
- **PhotosPerClump**: Photos in each cluster
- **SpreadKm**: Cluster radius in kilometers
- **Seed**: Random seed for reproducibility (fixed at 42)

Coordinates are distributed:
- Latitude: -60° to 60° (avoiding extreme polar regions)
- Longitude: -180° to 180° (full range)
- Each clump has Gaussian-like distribution around its center

## Performance Considerations

The map layout algorithm:
1. Uses physics simulation with up to 1000 iterations
2. Employs R-tree spatial indexing for collision detection
3. Scales roughly O(n log n) for well-distributed data
4. Performance degrades with very tight clusters (more collisions)

Key factors affecting performance:
- **Number of photos**: Primary driver of computation time
- **Spatial density**: Tighter clusters = more collision iterations
- **Number of clumps**: More dispersed = faster convergence
- **Spread radius**: Very tight spreads slow down collision resolution

## Profiling

To generate CPU profile:
```bash
go test -bench=BenchmarkLayoutMap_100Clumps_100Photos \
  -benchtime=10x \
  -cpuprofile=cpu.prof \
  ./internal/layout/

go tool pprof -http=:8080 cpu.prof
```

To generate memory profile:
```bash
go test -bench=BenchmarkLayoutMap_100Clumps_100Photos \
  -benchtime=10x \
  -memprofile=mem.prof \
  ./internal/layout/

go tool pprof -http=:8080 mem.prof
```

## CI/CD Integration

To prevent performance regressions, you can:

1. Add to CI pipeline with threshold checking:
```bash
go test -bench=BenchmarkLayoutMap_10Clumps_100Photos \
  -benchtime=5x ./internal/layout/ | \
  tee bench.txt

# Check if performance is within acceptable range
# (implementation depends on your CI tools)
```

2. Use benchstat in pull requests to compare performance
3. Track benchmark trends over time with continuous benchmarking tools
