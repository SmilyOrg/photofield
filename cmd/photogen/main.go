package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// Parameters for image generation
var (
	width      int
	height     int
	count      int
	outputDir  string
	quality    int
	seed       int64
	workers    int
	generators []generator
)

// Generator function type
type generator func(img draw.Image, number int, r *rand.Rand)

func init() {
	// Register generators
	generators = []generator{
		generateGradient,
		generateCheckered,
		generateConcentric,
		generateStripes,
	}

	// Parse command line flags
	flag.IntVar(&width, "width", 800, "Width of the generated images")
	flag.IntVar(&height, "height", 600, "Height of the generated images")
	flag.IntVar(&count, "count", 10, "Number of images to generate")
	flag.StringVar(&outputDir, "output", "output", "Output directory for images")
	flag.IntVar(&quality, "quality", 90, "JPEG quality (1-100)")
	flag.Int64Var(&seed, "seed", time.Now().UnixNano(), "Random seed")
	flag.IntVar(&workers, "workers", runtime.NumCPU(), "Number of worker goroutines")
}
func main() {
	flag.Parse()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Check if parameters have changed
	paramsFile := filepath.Join(outputDir, "params.txt")
	currentParams := fmt.Sprintf("width=%d\nheight=%d\ncount=%d\nquality=%d\nseed=%d\nworkers=%d",
		width, height, count, quality, seed, workers)

	if shouldSkipGeneration(paramsFile, currentParams) {
		fmt.Println("Parameters unchanged. Skipping generation.")
		return
	}

	// Initialize random number generator with main seed
	mainRand := rand.New(rand.NewSource(seed))

	if workers <= 0 {
		workers = 1
	}

	out := make(chan int, 100) // Buffered channel for worker output

	// Start worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)

		// Each worker gets its own random source derived from the main seed
		workerSeed := mainRand.Int63()
		go worker(w, workerSeed, (w*count)/workers, ((w+1)*count)/workers-1, &wg, out)
	}

	// Collect results from workers
	go func() {
		// Track when we last printed an update
		lastPrint := time.Now()
		gen := 0

		for range out {
			// Only print once per second
			if time.Since(lastPrint) >= time.Second {
				fmt.Printf("Generated image %6d / %6d\n", gen, count)
				lastPrint = time.Now()
			}
			gen++
		}
	}()

	// Wait for all workers to finish
	wg.Wait()

	// Close the output channel
	close(out)

	// Save parameters to file
	if err := os.WriteFile(paramsFile, []byte(currentParams), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not save parameters: %v\n", err)
	}

	fmt.Printf("Generated %d images in %s\n", count, outputDir)
	fmt.Println("Done!")
}

// Check if we should skip generation based on parameter changes
func shouldSkipGeneration(paramsFile, currentParams string) bool {
	// Check if params file exists
	prevParams, err := os.ReadFile(paramsFile)
	if err != nil {
		// If file doesn't exist or can't be read, generate new images
		return false
	}

	// Check if parameters match
	if string(prevParams) == currentParams {
		// Check if we have all the images
		pattern := filepath.Join(outputDir, "image_*.jpg")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return false
		}

		// If we have the right number of images, we can skip generation
		return len(matches) >= count
	}

	return false
}

// Worker function that processes jobs
func worker(id int, seed int64, start int, end int, wg *sync.WaitGroup, out chan<- int) {
	defer wg.Done()

	// Each worker has its own random generator
	r := rand.New(rand.NewSource(seed))

	for j := start; j <= end; j++ {
		// Create a new RGBA image
		img := image.NewRGBA(image.Rect(0, 0, width, height))

		// Choose a random generator
		generatorIndex := r.Intn(len(generators))
		generators[generatorIndex](img, j, r)

		// Draw number in the center
		drawNumber(img, j)

		// Save image
		filename := filepath.Join(outputDir, fmt.Sprintf("image_%04d.jpg", j))
		if err := saveJPEG(img, filename, quality); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving image %d: %v\n", j, err)
			continue
		}

		out <- j // Send the image number to the channel
	}
}

// Generator 1: Gradient patterns
func generateGradient(img draw.Image, number int, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Random gradient type: 0=linear, 1=radial, 2=angular
	gradientType := r.Intn(3)

	// Generate random colors for the gradient
	startColor := randomColor(r)
	endColor := randomColor(r)

	// Generate the gradient
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var t float64

			switch gradientType {
			case 0: // Linear gradient
				// Random direction for the gradient
				angle := r.Float64() * 2 * math.Pi
				nx := float64(x) / float64(w)
				ny := float64(y) / float64(h)
				// Project point onto angle direction
				t = nx*math.Cos(angle) + ny*math.Sin(angle)

			case 1: // Radial gradient
				centerX := float64(w) * r.Float64()
				centerY := float64(h) * r.Float64()
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				// Distance from center, normalized
				dist := math.Sqrt(dx*dx+dy*dy) / math.Sqrt(float64(w*w+h*h)) * 2
				t = math.Min(dist, 1.0)

			case 2: // Angular gradient
				centerX := float64(w) * r.Float64()
				centerY := float64(h) * r.Float64()
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				// Angle from center
				angle := math.Atan2(dy, dx)
				t = (angle + math.Pi) / (2 * math.Pi)
			}

			// Interpolate between start and end colors
			img.Set(x, y, interpolateColor(startColor, endColor, t))
		}
	}
}

// Generator 2: Checkered patterns
func generateCheckered(img draw.Image, number int, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Random cell sizes between 10 and 50 pixels
	cellWidth := 10 + r.Intn(41)
	cellHeight := 10 + r.Intn(41)

	// Generate 2-8 colors for the pattern
	numColors := 2 + r.Intn(7)
	colors := make([]color.RGBA, numColors)
	for i := range colors {
		colors[i] = randomColor(r)
	}

	// Optional offset for more interesting patterns
	offsetX := r.Intn(cellWidth)
	offsetY := r.Intn(cellHeight)

	// Fill the image with a checkered pattern
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gridX := (x + offsetX) / cellWidth
			gridY := (y + offsetY) / cellHeight
			colorIndex := (gridX + gridY) % numColors
			if r.Intn(5) == 0 { // 20% chance to add some randomness
				colorIndex = r.Intn(numColors)
			}
			img.Set(x, y, colors[colorIndex])
		}
	}
}

// Generator 3: Concentric shapes
func generateConcentric(img draw.Image, number int, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	centerX := w / 2
	centerY := h / 2

	// Background color
	bgColor := randomColor(r)
	draw.Draw(img, bounds, &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Shape type: 0=circles, 1=squares, 2=triangles
	shapeType := r.Intn(3)

	// Number of shapes (5-30)
	numShapes := 5 + r.Intn(26)

	// Max radius is half the minimum dimension
	maxRadius := int(math.Min(float64(w), float64(h)) / 2)
	radiusStep := maxRadius / numShapes

	// Generate a gradient of colors
	startColor := randomColor(r)
	endColor := randomColor(r)

	// Draw shapes from largest to smallest
	for i := numShapes - 1; i >= 0; i-- {
		radius := (i + 1) * radiusStep
		t := float64(i) / float64(numShapes-1)
		c := interpolateColor(startColor, endColor, t)

		switch shapeType {
		case 0: // Circles
			drawCircle(img, centerX, centerY, radius, c)
		case 1: // Squares
			drawSquare(img, centerX, centerY, radius, c)
		case 2: // Triangles
			drawPolygon(img, centerX, centerY, radius, 3, c)
		}
	}
}

// Generator 4: Striped patterns
func generateStripes(img draw.Image, number int, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Background color
	bgColor := randomColor(r)
	draw.Draw(img, bounds, &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Stripe type: 0=horizontal, 1=vertical, 2=diagonal
	stripeType := r.Intn(3)

	// Number of stripes (5-50)
	numStripes := 5 + r.Intn(46)

	// Stripe widths (variable or fixed)
	variableWidth := r.Intn(2) == 0
	baseWidth := int(math.Min(float64(w), float64(h))) / numStripes
	if baseWidth < 2 {
		baseWidth = 2
	}

	// Generate 2-5 colors for stripes
	numColors := 2 + r.Intn(4)
	colors := make([]color.RGBA, numColors)
	for i := range colors {
		colors[i] = randomColor(r)
	}

	// Draw stripes
	for i := 0; i < numStripes; i++ {
		// Calculate stripe dimensions
		stripeWidth := baseWidth
		if variableWidth {
			stripeWidth = baseWidth/2 + r.Intn(baseWidth)
		}

		// Select color
		stripeColor := colors[i%numColors]

		// Determine stripe position
		pos := i * baseWidth

		switch stripeType {
		case 0: // Horizontal stripes
			for y := pos; y < pos+stripeWidth && y < h; y++ {
				for x := 0; x < w; x++ {
					img.Set(x, y, stripeColor)
				}
			}

		case 1: // Vertical stripes
			for x := pos; x < pos+stripeWidth && x < w; x++ {
				for y := 0; y < h; y++ {
					img.Set(x, y, stripeColor)
				}
			}

		case 2: // Diagonal stripes
			// Determine diagonal direction: 0=top-left to bottom-right, 1=top-right to bottom-left
			diagonalDirection := r.Intn(2)

			// Calculate diagonal parameters
			diagonalWidth := int(math.Sqrt(float64(w*w + h*h)))
			diagonalStep := diagonalWidth / numStripes

			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					// Calculate position along the diagonal direction
					var diagonalPos int
					if diagonalDirection == 0 {
						diagonalPos = x + y // top-left to bottom-right
					} else {
						diagonalPos = x - y + h // top-right to bottom-left
					}

					// Determine which stripe this pixel belongs to
					stripeIndex := diagonalPos / diagonalStep

					// Get the color for this stripe
					color := colors[stripeIndex%numColors]

					// Apply color
					img.Set(x, y, color)
				}
			}
		}
	}

	// Add some noise for texture (10% of pixels)
	if r.Intn(2) == 0 {
		noiseAmount := 0.05 + r.Float64()*0.1 // 5-15% noise
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if r.Float64() < noiseAmount {
					img.Set(x, y, colors[r.Intn(numColors)])
				}
			}
		}
	}
}

// Draw a circle
func drawCircle(img draw.Image, centerX, centerY, radius int, c color.RGBA) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := x - centerX
			dy := y - centerY
			dist := math.Sqrt(float64(dx*dx + dy*dy))
			if dist <= float64(radius) {
				img.Set(x, y, c)
			}
		}
	}
}

// Draw a square
func drawSquare(img draw.Image, centerX, centerY, radius int, c color.RGBA) {
	minX := centerX - radius
	maxX := centerX + radius
	minY := centerY - radius
	maxY := centerY + radius

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if x >= 0 && y >= 0 && x < img.Bounds().Dx() && y < img.Bounds().Dy() {
				img.Set(x, y, c)
			}
		}
	}
}

// Draw a regular polygon
func drawPolygon(img draw.Image, centerX, centerY, radius int, sides int, c color.RGBA) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Calculate vertices
	vertices := make([]image.Point, sides)
	for i := 0; i < sides; i++ {
		angle := 2*math.Pi*float64(i)/float64(sides) - math.Pi/2
		x := centerX + int(float64(radius)*math.Cos(angle))
		y := centerY + int(float64(radius)*math.Sin(angle))
		vertices[i] = image.Point{X: x, Y: y}
	}

	// Simple polygon filling algorithm
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pointInPolygon(x, y, vertices) {
				img.Set(x, y, c)
			}
		}
	}
}

// Check if a point is inside a polygon
func pointInPolygon(x, y int, vertices []image.Point) bool {
	inside := false
	j := len(vertices) - 1

	for i := 0; i < len(vertices); i++ {
		if ((vertices[i].Y > y) != (vertices[j].Y > y)) &&
			(x < (vertices[j].X-vertices[i].X)*(y-vertices[i].Y)/(vertices[j].Y-vertices[i].Y)+vertices[i].X) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// Draw the image number in the center
func drawNumber(img draw.Image, number int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	centerX := w / 2
	centerY := h / 2

	// Convert number to string
	text := strconv.Itoa(number)

	// Simple bitmap font size
	fontWidth := w / 10
	fontHeight := h / 5
	thickness := int(math.Max(float64(fontWidth)/8, 3))

	// Text color (contrasting with background)
	var textColor color.RGBA
	bgR, bgG, bgB, _ := img.At(centerX, centerY).RGBA()
	brightness := (bgR + bgG + bgB) / 3
	if brightness > 0x8000 {
		textColor = color.RGBA{0, 0, 0, 255} // Black text on light background
	} else {
		textColor = color.RGBA{255, 255, 255, 255} // White text on dark background
	}

	// Draw each digit
	digitWidth := fontWidth * 1000 / 900
	totalWidth := len(text) * digitWidth
	startX := centerX - totalWidth/2

	for i, char := range text {
		digit := int(char - '0')
		drawDigit(img, startX+i*digitWidth, centerY-fontHeight/2, fontWidth, fontHeight, thickness, digit, textColor)
	}
}

// Draw a single digit
func drawDigit(img draw.Image, x, y, width, height, thickness, digit int, color color.RGBA) {
	// Simple 7-segment display logic
	segments := []bool{
		// Segments a, b, c, d, e, f, g for each digit 0-9
		true, true, true, true, true, true, false, // 0
		false, true, true, false, false, false, false, // 1
		true, true, false, true, true, false, true, // 2
		true, true, true, true, false, false, true, // 3
		false, true, true, false, false, true, true, // 4
		true, false, true, true, false, true, true, // 5
		true, false, true, true, true, true, true, // 6
		true, true, true, false, false, false, false, // 7
		true, true, true, true, true, true, true, // 8
		true, true, true, true, false, true, true, // 9
	}

	// Segment positions (a, b, c, d, e, f, g)
	segmentStart := digit * 7

	// Draw the segments
	if segments[segmentStart] { // a (top horizontal)
		drawHorizontalSegment(img, x, y, width, thickness, color)
	}
	if segments[segmentStart+1] { // b (top right vertical)
		drawVerticalSegment(img, x+width-thickness, y, height/2, thickness, color)
	}
	if segments[segmentStart+2] { // c (bottom right vertical)
		drawVerticalSegment(img, x+width-thickness, y+height/2, height/2, thickness, color)
	}
	if segments[segmentStart+3] { // d (bottom horizontal)
		drawHorizontalSegment(img, x, y+height-thickness, width, thickness, color)
	}
	if segments[segmentStart+4] { // e (bottom left vertical)
		drawVerticalSegment(img, x, y+height/2, height/2, thickness, color)
	}
	if segments[segmentStart+5] { // f (top left vertical)
		drawVerticalSegment(img, x, y, height/2, thickness, color)
	}
	if segments[segmentStart+6] { // g (middle horizontal)
		drawHorizontalSegment(img, x, y+height/2-thickness/2, width, thickness, color)
	}
}

// Draw a horizontal segment
func drawHorizontalSegment(img draw.Image, x, y, width, thickness int, color color.RGBA) {
	for dy := 0; dy < thickness; dy++ {
		for dx := 0; dx < width; dx++ {
			if x+dx >= 0 && y+dy >= 0 && x+dx < img.Bounds().Dx() && y+dy < img.Bounds().Dy() {
				img.Set(x+dx, y+dy, color)
			}
		}
	}
}

// Draw a vertical segment
func drawVerticalSegment(img draw.Image, x, y, height, thickness int, color color.RGBA) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < thickness; dx++ {
			if x+dx >= 0 && y+dy >= 0 && x+dx < img.Bounds().Dx() && y+dy < img.Bounds().Dy() {
				img.Set(x+dx, y+dy, color)
			}
		}
	}
}

// Generate a random color
func randomColor(r *rand.Rand) color.RGBA {
	return color.RGBA{
		R: uint8(r.Intn(256)),
		G: uint8(r.Intn(256)),
		B: uint8(r.Intn(256)),
		A: 255,
	}
}

// Interpolate between two colors
func interpolateColor(c1, c2 color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c1.R)*(1-t) + float64(c2.R)*t),
		G: uint8(float64(c1.G)*(1-t) + float64(c2.G)*t),
		B: uint8(float64(c1.B)*(1-t) + float64(c2.B)*t),
		A: 255,
	}
}

// Save image as JPEG
func saveJPEG(img image.Image, filename string, quality int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	opt := jpeg.Options{Quality: quality}
	return jpeg.Encode(file, img, &opt)
}
