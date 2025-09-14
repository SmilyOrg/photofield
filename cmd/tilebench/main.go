package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
)

type TileStats struct {
	Count         int
	Sizes         []int64
	Latencies     []time.Duration
	MinSize       int64
	MaxSize       int64
	MeanSize      float64
	MedianSize    int64
	MinLatency    time.Duration
	MaxLatency    time.Duration
	MeanLatency   time.Duration
	MedianLatency time.Duration
	TotalBytes    int64
	TotalTime     time.Duration
}

func main() {
	var (
		baseURL    = flag.String("url", "http://localhost:8080", "Base URL of the photofield server")
		sceneID    = flag.String("scene", "", "Scene ID (required)")
		tileSize   = flag.Int("tile-size", 512, "Tile size")
		zoom       = flag.Int("zoom", 16, "Zoom level")
		minX       = flag.Int("min-x", 0, "Minimum X coordinate")
		maxX       = flag.Int("max-x", 0, "Maximum X coordinate")
		minY       = flag.Int("min-y", 0, "Minimum Y coordinate")
		maxY       = flag.Int("max-y", 0, "Maximum Y coordinate")
		workers    = flag.Int("workers", 10, "Number of concurrent workers")
		acceptType = flag.String("accept", "", "Accept header / image type (e.g., image/webp, image/jpeg, image/png)")
		csvOutput  = flag.Bool("csv", false, "Output individual tile stats as CSV to stdout")
	)
	flag.Parse()

	if *sceneID == "" {
		log.Fatal("Scene ID is required (use -scene flag)")
	}

	if *maxX <= *minX || *maxY <= *minY {
		log.Fatal("Invalid coordinate ranges: max values must be greater than min values")
	}

	fmt.Fprintf(os.Stderr, "Benchmarking tiles for scene %s\n", *sceneID)
	fmt.Fprintf(os.Stderr, "Zoom: %d, Tile size: %d\n", *zoom, *tileSize)
	fmt.Fprintf(os.Stderr, "X range: %d-%d, Y range: %d-%d\n", *minX, *maxX, *minY, *maxY)
	fmt.Fprintf(os.Stderr, "Workers: %d\n", *workers)
	if *acceptType != "" {
		fmt.Fprintf(os.Stderr, "Accept header: %s\n", *acceptType)
	}
	fmt.Fprintln(os.Stderr)

	// if *csvOutput {
	// 	// Output CSV header to stdout
	// 	fmt.Println("x,y,size_bytes,latency_ms,accept_type,workers,error")
	// }

	stats := benchmarkTiles(*baseURL, *sceneID, *tileSize, *zoom, *minX, *maxX, *minY, *maxY, *workers, *acceptType, *csvOutput)

	if !*csvOutput {
		printStats(stats)
	}
}

type TileRequest struct {
	URL        string
	X          int
	Y          int
	AcceptType string
}

func benchmarkTiles(baseURL, sceneID string, tileSize, zoom, minX, maxX, minY, maxY, workers int, acceptType string, csvOutput bool) TileStats {
	// Generate all tile requests
	var requests []TileRequest
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			url := fmt.Sprintf("%s/scenes/%s/tiles?tile_size=%d&zoom=%d&x=%d&y=%d",
				baseURL, sceneID, tileSize, zoom, x, y)
			requests = append(requests, TileRequest{URL: url, X: x, Y: y, AcceptType: acceptType})
		}
	}

	totalTiles := len(requests)
	fmt.Fprintf(os.Stderr, "Total tiles to fetch: %d\n", totalTiles)

	// Create channels for work distribution
	requestChan := make(chan TileRequest, totalTiles)
	resultChan := make(chan TileResult, totalTiles)

	// Send all requests to channel
	for _, req := range requests {
		requestChan <- req
	}
	close(requestChan)

	// Start workers
	for i := 0; i < workers; i++ {
		go worker(requestChan, resultChan, csvOutput, workers)
	}

	// Collect results
	var sizes []int64
	var latencies []time.Duration
	var totalBytes int64
	var totalTime time.Duration
	var errors int

	startTime := time.Now()

	for i := 0; i < totalTiles; i++ {
		result := <-resultChan
		if result.Error != nil {
			errors++
			if !csvOutput {
				fmt.Fprintf(os.Stderr, "Error fetching tile %d,%d: %v\n", result.X, result.Y, result.Error)
			}
			continue
		}

		sizes = append(sizes, result.Size)
		latencies = append(latencies, result.Latency)
		totalBytes += result.Size
		totalTime += result.Latency

		// Progress indicator
		if (i+1)%100 == 0 || i+1 == totalTiles {
			fmt.Fprintf(os.Stderr, "Progress: %d/%d tiles completed\n", i+1, totalTiles)
		}
	}

	actualFetchTime := time.Since(startTime)

	if errors > 0 && !csvOutput {
		fmt.Fprintf(os.Stderr, "Encountered %d errors during fetching\n", errors)
	}

	// Calculate statistics
	stats := TileStats{
		Count:      len(sizes),
		Sizes:      sizes,
		Latencies:  latencies,
		TotalBytes: totalBytes,
		TotalTime:  totalTime,
	}

	if len(sizes) > 0 {
		// Sort for median calculation
		sortedSizes := make([]int64, len(sizes))
		copy(sortedSizes, sizes)
		sort.Slice(sortedSizes, func(i, j int) bool { return sortedSizes[i] < sortedSizes[j] })

		sortedLatencies := make([]time.Duration, len(latencies))
		copy(sortedLatencies, latencies)
		sort.Slice(sortedLatencies, func(i, j int) bool { return sortedLatencies[i] < sortedLatencies[j] })

		// Size statistics
		stats.MinSize = sortedSizes[0]
		stats.MaxSize = sortedSizes[len(sortedSizes)-1]
		stats.MeanSize = float64(totalBytes) / float64(len(sizes))
		if len(sortedSizes)%2 == 0 {
			stats.MedianSize = (sortedSizes[len(sortedSizes)/2-1] + sortedSizes[len(sortedSizes)/2]) / 2
		} else {
			stats.MedianSize = sortedSizes[len(sortedSizes)/2]
		}

		// Latency statistics
		stats.MinLatency = sortedLatencies[0]
		stats.MaxLatency = sortedLatencies[len(sortedLatencies)-1]
		stats.MeanLatency = totalTime / time.Duration(len(latencies))
		if len(sortedLatencies)%2 == 0 {
			stats.MedianLatency = (sortedLatencies[len(sortedLatencies)/2-1] + sortedLatencies[len(sortedLatencies)/2]) / 2
		} else {
			stats.MedianLatency = sortedLatencies[len(sortedLatencies)/2]
		}
	}

	if !csvOutput {
		fmt.Fprintf(os.Stderr, "\nTotal benchmark time: %v\n", actualFetchTime)
	}
	return stats
}

type TileResult struct {
	Size       int64
	Latency    time.Duration
	X          int
	Y          int
	AcceptType string
	Error      error
}

func worker(requests <-chan TileRequest, results chan<- TileResult, csvOutput bool, workers int) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for req := range requests {
		start := time.Now()

		httpReq, err := http.NewRequest("GET", req.URL, nil)
		if err != nil {
			result := TileResult{X: req.X, Y: req.Y, AcceptType: req.AcceptType, Error: err}
			if csvOutput {
				fmt.Printf("%d,%d,,,%s,%d,%s\n", req.X, req.Y, req.AcceptType, workers, err.Error())
			}
			results <- result
			continue
		}

		// Set Accept header if specified
		if req.AcceptType != "" {
			httpReq.Header.Set("Accept", req.AcceptType)
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			result := TileResult{X: req.X, Y: req.Y, AcceptType: req.AcceptType, Error: err}
			if csvOutput {
				fmt.Printf("%d,%d,,,%s,%d,%s\n", req.X, req.Y, req.AcceptType, workers, err.Error())
			}
			results <- result
			continue
		}

		body, err := io.ReadAll(resp.Body)
		size := len(body)
		resp.Body.Close()

		latency := time.Since(start)

		if err != nil {
			result := TileResult{X: req.X, Y: req.Y, AcceptType: req.AcceptType, Error: err}
			if csvOutput {
				fmt.Printf("%d,%d,,,%s,%d,%s\n", req.X, req.Y, req.AcceptType, workers, err.Error())
			}
			results <- result
			continue
		}

		if resp.StatusCode != http.StatusOK {
			httpErr := fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			result := TileResult{
				X:          req.X,
				Y:          req.Y,
				AcceptType: req.AcceptType,
				Error:      httpErr,
			}
			if csvOutput {
				fmt.Printf("%d,%d,,,%s,%d,%s\n", req.X, req.Y, req.AcceptType, workers, httpErr.Error())
			}
			results <- result
			continue
		}

		// fmt.Fprintf(os.Stderr, "fetch %6d %6d %6d %s\n", req.X, req.Y, size, req.AcceptType)

		result := TileResult{
			Size:       int64(size),
			Latency:    latency,
			X:          req.X,
			Y:          req.Y,
			AcceptType: req.AcceptType,
		}

		if csvOutput {
			fmt.Printf("%d,%d,%d,%.2f,%s,%d,\n", req.X, req.Y, result.Size, float64(latency.Nanoseconds())/1000000.0, req.AcceptType, workers)
		}

		results <- result
	}
}

func printStats(stats TileStats) {
	fmt.Fprintln(os.Stderr, "\n=== TILE BENCHMARK RESULTS ===")
	fmt.Fprintf(os.Stderr, "Successfully fetched tiles: %d\n", stats.Count)

	if stats.Count == 0 {
		fmt.Fprintln(os.Stderr, "No tiles were successfully fetched")
		return
	}

	fmt.Fprintln(os.Stderr, "\n--- SIZE STATISTICS ---")
	fmt.Fprintf(os.Stderr, "Total bytes: %s (%d bytes)\n", formatBytes(stats.TotalBytes), stats.TotalBytes)
	fmt.Fprintf(os.Stderr, "Min size: %s (%d bytes)\n", formatBytes(stats.MinSize), stats.MinSize)
	fmt.Fprintf(os.Stderr, "Max size: %s (%d bytes)\n", formatBytes(stats.MaxSize), stats.MaxSize)
	fmt.Fprintf(os.Stderr, "Mean size: %s (%.1f bytes)\n", formatBytes(int64(stats.MeanSize)), stats.MeanSize)
	fmt.Fprintf(os.Stderr, "Median size: %s (%d bytes)\n", formatBytes(stats.MedianSize), stats.MedianSize)

	fmt.Fprintln(os.Stderr, "\n--- LATENCY STATISTICS ---")
	fmt.Fprintf(os.Stderr, "Total time: %v\n", stats.TotalTime)
	fmt.Fprintf(os.Stderr, "Min latency: %v\n", stats.MinLatency)
	fmt.Fprintf(os.Stderr, "Max latency: %v\n", stats.MaxLatency)
	fmt.Fprintf(os.Stderr, "Mean latency: %v\n", stats.MeanLatency)
	fmt.Fprintf(os.Stderr, "Median latency: %v\n", stats.MedianLatency)

	fmt.Fprintln(os.Stderr, "\n--- THROUGHPUT ---")
	if stats.MeanLatency > 0 {
		tilesPerSecond := float64(time.Second) / float64(stats.MeanLatency)
		bytesPerSecond := float64(stats.TotalBytes) * tilesPerSecond / float64(stats.Count)
		fmt.Fprintf(os.Stderr, "Tiles per second (avg): %.2f\n", tilesPerSecond)
		fmt.Fprintf(os.Stderr, "Throughput (avg): %s/s\n", formatBytes(int64(bytesPerSecond)))
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
