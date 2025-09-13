package test

import (
	"crypto/md5"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// ImageSpec defines the parameters for a single test image
type ImageSpec struct {
	Width    int               // Image width in pixels
	Height   int               // Image height in pixels
	ExifTags map[string]string // EXIF tags to set (key-value pairs)
}

// TestDataset represents a collection of test images
type TestDataset struct {
	Name    string      // Dataset name (used as subdirectory)
	Images  []ImageSpec // List of images to generate
	Samples int         // Number of samples to generate per ImageSpec
	Seed    int64       // Random seed for reproducible generation
}

// GeneratedImage represents a generated test image
type GeneratedImage struct {
	Name string // Auto-generated image name
	Path string // Full path to the image file
	Spec ImageSpec
}

// ImageSize represents image dimensions for pool keys
type ImageSize struct {
	Width  int
	Height int
}

// SizedImagePool manages reusable image buffers per size to reduce allocations
type SizedImagePool struct {
	pools sync.Map // map[ImageSize]*sync.Pool
}

// NewSizedImagePool creates a new size-specific image pool
func NewSizedImagePool() *SizedImagePool {
	return &SizedImagePool{}
}

// getPoolForSize gets or creates a pool for the specific image size
func (p *SizedImagePool) getPoolForSize(width, height int) *sync.Pool {
	size := ImageSize{Width: width, Height: height}

	if pool, ok := p.pools.Load(size); ok {
		return pool.(*sync.Pool)
	}

	// Create new pool for this size
	newPool := &sync.Pool{
		New: func() interface{} {
			return image.NewRGBA(image.Rect(0, 0, width, height))
		},
	}

	// Store the pool (another goroutine might have created one in the meantime, but that's fine)
	actual, _ := p.pools.LoadOrStore(size, newPool)
	return actual.(*sync.Pool)
}

// Get retrieves an image from the size-specific pool
func (p *SizedImagePool) Get(width, height int) *image.RGBA {
	pool := p.getPoolForSize(width, height)
	img := pool.Get().(*image.RGBA)

	// Clear the image to transparent black
	for i := range img.Pix {
		img.Pix[i] = 0
	}

	return img
}

// Put returns an image to the appropriate size-specific pool
func (p *SizedImagePool) Put(img *image.RGBA) {
	if img == nil {
		return
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	pool := p.getPoolForSize(width, height)
	pool.Put(img)
}

// TestImageGenerator manages test image generation
type TestImageGenerator struct {
	baseDir    string
	generators []generator
	imagePool  *SizedImagePool
}

// Generator function type
type generator func(img draw.Image, r *rand.Rand)

// GenerateTestDataset creates test images based on the dataset specification
// This is the main entry point that combines generator creation and dataset generation
func GenerateTestDataset(baseDir string, dataset TestDataset) ([]GeneratedImage, error) {
	if baseDir == "" {
		baseDir = "testdata"
	}

	g := &TestImageGenerator{
		baseDir:   baseDir,
		imagePool: NewSizedImagePool(),
		generators: []generator{
			// generateGradient,
			generateCheckered,
			// generateConcentric,
			generateStripes,
		},
	}

	return g.GenerateDataset(dataset)
}

// TransformCornersByName applies the given orientation transformation to four corner colors
func TransformCornersByName(tl, tr, bl, br color.Color, orientationName string) (color.Color, color.Color, color.Color, color.Color) {
	switch orientationName {
	case "Horizontal (normal)", "":
		// No transformation
		return tl, tr, bl, br
	case "Mirror horizontal":
		// Flip horizontally: left becomes right, right becomes left
		return tr, tl, br, bl
	case "Rotate 180":
		// Rotate 180°: top-left becomes bottom-right, etc.
		return br, bl, tr, tl
	case "Mirror vertical":
		// Flip vertically: top becomes bottom, bottom becomes top
		return bl, br, tl, tr
	case "Mirror horizontal and rotate 270 CW":
		// Mirror horizontal then rotate 270° CW (equivalent to transpose)
		return tl, bl, tr, br
	case "Rotate 90 CW":
		// Rotate 90° clockwise: top-left becomes top-right, etc.
		return bl, tl, br, tr
	case "Mirror horizontal and rotate 90 CW":
		// Mirror horizontal then rotate 90° CW (equivalent to anti-transpose)
		return br, tr, bl, tl
	case "Rotate 270 CW":
		// Rotate 270° clockwise: top-left becomes bottom-left, etc.
		return tr, br, tl, bl
	default:
		// Unknown orientation, return as-is
		return tl, tr, bl, br
	}
}

// TransformCornersByName applies the given orientation transformation to four corner colors
func TransformCornersByNumber(tl, tr, bl, br color.Color, orientationNumber int) (color.Color, color.Color, color.Color, color.Color) {
	switch orientationNumber {
	case 1:
		// No transformation
		return tl, tr, bl, br
	case 2:
		// Flip horizontally: left becomes right, right becomes left
		return tr, tl, br, bl
	case 3:
		// Rotate 180°: top-left becomes bottom-right, etc.
		return br, bl, tr, tl
	case 4:
		// Flip vertically: top becomes bottom, bottom becomes top
		return bl, br, tl, tr
	case 5:
		// Mirror horizontal and rotate 270° CW (equivalent to transpose)
		return tl, bl, tr, br
	case 6:
		// Rotate 90° clockwise: top-left becomes top-right, etc.
		return bl, tl, br, tr
	case 7:
		// Mirror horizontal then rotate 90° CW (equivalent to anti-transpose)
		return br, tr, bl, tl
	case 8:
		// Rotate 270° clockwise: top-left becomes bottom-left, etc.
		return tr, br, tl, bl
	default:
		// Unknown orientation, return as-is
		return tl, tr, bl, br
	}
}

// ColorsSimilar compares two color values for approximate equality
func ColorsSimilar(c1, c2 color.Color, tolerance int) bool {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()

	tolerance *= 1024

	return abs(int(r1)-int(r2)) <= tolerance &&
		abs(int(g1)-int(g2)) <= tolerance &&
		abs(int(b1)-int(b2)) <= tolerance &&
		abs(int(a1)-int(a2)) <= tolerance
}

func ColorDiff(c1, c2 color.Color) int {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()
	return (abs(int(r1)-int(r2)) +
		abs(int(g1)-int(g2)) +
		abs(int(b1)-int(b2)) +
		abs(int(a1)-int(a2))) / 0xFF
}

func ColorToInt(c color.Color) int {
	r, _, _, _ := c.RGBA()
	return int(r) / 0xFF
}

// abs returns the absolute value of x
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// generateImageName creates a unique name based on image parameters
func generateImageName(spec ImageSpec, specIndex, sampleIndex int) string {
	// Create base name from dimensions
	baseName := fmt.Sprintf("img_%dx%d", spec.Width, spec.Height)

	// Add EXIF tag hash if present
	if len(spec.ExifTags) > 0 {
		// Sort keys for consistent hashing
		keys := make([]string, 0, len(spec.ExifTags))
		for k := range spec.ExifTags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var tagStr strings.Builder
		for _, k := range keys {
			tagStr.WriteString(k)
			tagStr.WriteString("=")
			tagStr.WriteString(spec.ExifTags[k])
			tagStr.WriteString(";")
		}

		hash := md5.Sum([]byte(tagStr.String()))
		baseName += fmt.Sprintf("_%x", hash[:4])
	}

	// Add spec and sample indices to ensure uniqueness
	return fmt.Sprintf("%s_%03d_%03d", baseName, specIndex, sampleIndex)
}

// GenerateDataset creates test images based on the dataset specification
func (g *TestImageGenerator) GenerateDataset(dataset TestDataset) ([]GeneratedImage, error) {
	datasetDir := filepath.Join(g.baseDir, dataset.Name)

	// Check if dataset already exists and is up to date
	if g.isDatasetCurrent(datasetDir, dataset) {
		return g.loadExistingDataset(datasetDir, dataset)
	}

	// Create dataset directory
	if err := os.MkdirAll(datasetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create dataset directory: %w", err)
	}

	// Generate images
	images, err := g.generateImages(datasetDir, dataset)
	if err != nil {
		return nil, fmt.Errorf("failed to generate images: %w", err)
	}

	// Set EXIF tags
	if err := g.setExifTags(images); err != nil {
		return nil, fmt.Errorf("failed to set EXIF tags: %w", err)
	}

	// Save dataset metadata
	if err := g.saveDatasetMetadata(datasetDir, dataset); err != nil {
		return nil, fmt.Errorf("failed to save dataset metadata: %w", err)
	}

	return images, nil
}

// isDatasetCurrent checks if the dataset exists and matches the specification
func (g *TestImageGenerator) isDatasetCurrent(datasetDir string, dataset TestDataset) bool {
	metadataFile := filepath.Join(datasetDir, "metadata.txt")

	// Check if metadata file exists
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		return false
	}

	// Check if all image files exist
	for i, spec := range dataset.Images {
		for j := 0; j < dataset.Samples; j++ {
			imageName := generateImageName(spec, i, j)
			imagePath := filepath.Join(datasetDir, imageName+".jpg")
			if _, err := os.Stat(imagePath); os.IsNotExist(err) {
				return false
			}
		}
	}

	// Read existing metadata
	content, err := os.ReadFile(metadataFile)
	if err != nil {
		return false
	}

	expectedMetadata := g.generateMetadata(dataset)
	return string(content) == expectedMetadata
}

// loadExistingDataset loads information about existing dataset
func (g *TestImageGenerator) loadExistingDataset(datasetDir string, dataset TestDataset) ([]GeneratedImage, error) {
	totalImages := len(dataset.Images) * dataset.Samples
	images := make([]GeneratedImage, totalImages)

	imageIndex := 0
	for i, spec := range dataset.Images {
		for j := 0; j < dataset.Samples; j++ {
			imageName := generateImageName(spec, i, j)
			imagePath := filepath.Join(datasetDir, imageName+".jpg")
			images[imageIndex] = GeneratedImage{
				Name: imageName,
				Path: imagePath,
				Spec: spec,
			}
			imageIndex++
		}
	}

	return images, nil
}

// generateImages creates the actual image files
func (g *TestImageGenerator) generateImages(datasetDir string, dataset TestDataset) ([]GeneratedImage, error) {
	totalImages := len(dataset.Images) * dataset.Samples
	images := make([]GeneratedImage, totalImages)
	mainRand := rand.New(rand.NewSource(dataset.Seed))

	// Work item definition
	type workItem struct {
		specIndex   int
		sampleIndex int
		imageIndex  int
		spec        ImageSpec
		imageSeed   int64
	}

	// Worker pool setup
	concurrency := runtime.GOMAXPROCS(0)
	workChan := make(chan workItem, totalImages)
	errChan := make(chan error, totalImages)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for work := range workChan {
				// Generate image name
				imageName := generateImageName(work.spec, work.specIndex, work.sampleIndex)

				// Each image gets its own random source
				r := rand.New(rand.NewSource(work.imageSeed))

				// Get reusable image from pool
				img := g.imagePool.Get(work.spec.Width, work.spec.Height)

				// Choose random generator
				generatorIndex := r.Intn(len(g.generators))
				g.generators[generatorIndex](img, r)

				// Draw image name/number
				g.drawText(img, imageName)

				// Save image
				imagePath := filepath.Join(datasetDir, imageName+".jpg")
				if err := g.saveJPEG(img, imagePath); err != nil {
					errChan <- fmt.Errorf("failed to save image %s: %w", imageName, err)
					// Return image to pool even on error
					g.imagePool.Put(img)
					continue
				}

				// Return image to pool after use
				g.imagePool.Put(img)

				images[work.imageIndex] = GeneratedImage{
					Name: imageName,
					Path: imagePath,
					Spec: work.spec,
				}
			}
		}()
	}

	// Send work to workers
	imageIndex := 0
	for i, spec := range dataset.Images {
		for j := 0; j < dataset.Samples; j++ {
			imageSeed := mainRand.Int63()
			workChan <- workItem{
				specIndex:   i,
				sampleIndex: j,
				imageIndex:  imageIndex,
				spec:        spec,
				imageSeed:   imageSeed,
			}
			imageIndex++
		}
	}
	close(workChan)

	// Wait for workers to finish
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return images, nil
}

// setExifTags applies EXIF tags to generated images using exiftool
func (g *TestImageGenerator) setExifTags(images []GeneratedImage) error {
	for _, img := range images {
		if len(img.Spec.ExifTags) == 0 {
			continue
		}

		args := []string{"-overwrite_original"}
		for key, value := range img.Spec.ExifTags {
			args = append(args, fmt.Sprintf("-%s=%s", key, value))
		}
		args = append(args, img.Path)

		cmd := exec.Command("exiftool", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set EXIF tags for %s: %w", img.Name, err)
		}
	}

	return nil
}

// generateMetadata creates metadata string for the dataset
func (g *TestImageGenerator) generateMetadata(dataset TestDataset) string {
	totalImages := len(dataset.Images) * dataset.Samples
	metadata := fmt.Sprintf("name=%s\nseed=%d\nspecs=%d\nsamples=%d\nimages=%d\n",
		dataset.Name, dataset.Seed, len(dataset.Images), dataset.Samples, totalImages)

	for i, spec := range dataset.Images {
		for j := 0; j < dataset.Samples; j++ {
			imageName := generateImageName(spec, i, j)
			metadata += fmt.Sprintf("image=%s,%dx%d", imageName, spec.Width, spec.Height)
			// Write EXIF tags in sorted order for deterministic metadata
			if len(spec.ExifTags) > 0 {
				keys := make([]string, 0, len(spec.ExifTags))
				for key := range spec.ExifTags {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					metadata += fmt.Sprintf(",%s=%s", key, spec.ExifTags[key])
				}
			}
			metadata += "\n"
		}
	}

	return metadata
}

// saveDatasetMetadata saves dataset metadata to file
func (g *TestImageGenerator) saveDatasetMetadata(datasetDir string, dataset TestDataset) error {
	metadataFile := filepath.Join(datasetDir, "metadata.txt")
	metadata := g.generateMetadata(dataset)
	return os.WriteFile(metadataFile, []byte(metadata), 0644)
}

// Generator functions (simplified from original)
func generateGradient(img draw.Image, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	startColor := randomColor(r)
	endColor := randomColor(r)
	gradientType := r.Intn(3)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var t float64

			switch gradientType {
			case 0: // Linear
				angle := r.Float64() * 2 * math.Pi
				nx := float64(x) / float64(w)
				ny := float64(y) / float64(h)
				t = nx*math.Cos(angle) + ny*math.Sin(angle)
			case 1: // Radial
				centerX := float64(w) * r.Float64()
				centerY := float64(h) * r.Float64()
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				dist := math.Sqrt(dx*dx+dy*dy) / math.Sqrt(float64(w*w+h*h)) * 2
				t = math.Min(dist, 1.0)
			case 2: // Angular
				centerX := float64(w) * r.Float64()
				centerY := float64(h) * r.Float64()
				dx := float64(x) - centerX
				dy := float64(y) - centerY
				angle := math.Atan2(dy, dx)
				t = (angle + math.Pi) / (2 * math.Pi)
			}

			img.Set(x, y, interpolateColor(startColor, endColor, t))
		}
	}
}

func generateCheckered(img draw.Image, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	cellWidth := 10 + r.Intn(41)
	cellHeight := 10 + r.Intn(41)

	numColors := 2 + r.Intn(7)
	colors := make([]color.RGBA, numColors)
	for i := range colors {
		colors[i] = randomColor(r)
	}

	offsetX := r.Intn(cellWidth)
	offsetY := r.Intn(cellHeight)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gridX := (x + offsetX) / cellWidth
			gridY := (y + offsetY) / cellHeight
			colorIndex := (gridX + gridY) % numColors
			if r.Intn(5) == 0 {
				colorIndex = r.Intn(numColors)
			}
			img.Set(x, y, colors[colorIndex])
		}
	}
}

func generateConcentric(img draw.Image, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	centerX := w / 2
	centerY := h / 2

	bgColor := randomColor(r)
	draw.Draw(img, bounds, &image.Uniform{bgColor}, image.Point{}, draw.Src)

	shapeType := r.Intn(2) // Only circles and squares for simplicity
	numShapes := 5 + r.Intn(26)
	maxRadius := int(math.Min(float64(w), float64(h)) / 2)
	radiusStep := maxRadius / numShapes

	startColor := randomColor(r)
	endColor := randomColor(r)

	for i := numShapes - 1; i >= 0; i-- {
		radius := (i + 1) * radiusStep
		t := float64(i) / float64(numShapes-1)
		c := interpolateColor(startColor, endColor, t)

		switch shapeType {
		case 0:
			drawCircle(img, centerX, centerY, radius, c)
		case 1:
			drawSquare(img, centerX, centerY, radius, c)
		}
	}
}

func generateStripes(img draw.Image, r *rand.Rand) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	bgColor := randomColor(r)
	draw.Draw(img, bounds, &image.Uniform{bgColor}, image.Point{}, draw.Src)

	stripeType := r.Intn(2) // Only horizontal and vertical
	numStripes := 5 + r.Intn(46)

	numColors := 2 + r.Intn(4)
	colors := make([]color.RGBA, numColors)
	for i := range colors {
		colors[i] = randomColor(r)
	}

	var baseWidth int
	if stripeType == 0 {
		baseWidth = h / numStripes
	} else {
		baseWidth = w / numStripes
	}
	if baseWidth < 2 {
		baseWidth = 2
	}

	for i := 0; i < numStripes; i++ {
		stripeColor := colors[i%numColors]
		pos := i * baseWidth

		switch stripeType {
		case 0: // Horizontal
			for y := pos; y < pos+baseWidth && y < h; y++ {
				for x := 0; x < w; x++ {
					img.Set(x, y, stripeColor)
				}
			}
		case 1: // Vertical
			for x := pos; x < pos+baseWidth && x < w; x++ {
				for y := 0; y < h; y++ {
					img.Set(x, y, stripeColor)
				}
			}
		}
	}
}

// Helper functions
func (g *TestImageGenerator) drawText(img draw.Image, text string) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	centerX := w / 2
	centerY := h / 2

	fontWidth := w / 10
	fontHeight := h / 5
	thickness := int(math.Max(float64(fontWidth)/8, 3))

	var textColor color.RGBA
	bgR, bgG, bgB, _ := img.At(centerX, centerY).RGBA()
	brightness := (bgR + bgG + bgB) / 3
	if brightness > 0x8000 {
		textColor = color.RGBA{0, 0, 0, 255}
	} else {
		textColor = color.RGBA{255, 255, 255, 255}
	}

	digitWidth := fontWidth * 1000 / 900
	totalWidth := len(text) * digitWidth
	startX := centerX - totalWidth/2

	for i, char := range text {
		if char >= '0' && char <= '9' {
			digit := int(char - '0')
			drawDigit(img, startX+i*digitWidth, centerY-fontHeight/2, fontWidth, fontHeight, thickness, digit, textColor)
		}
	}
}

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

func drawDigit(img draw.Image, x, y, width, height, thickness, digit int, color color.RGBA) {
	segments := []bool{
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

	if digit < 0 || digit > 9 {
		return
	}

	segmentStart := digit * 7

	if segments[segmentStart] {
		drawHorizontalSegment(img, x, y, width, thickness, color)
	}
	if segments[segmentStart+1] {
		drawVerticalSegment(img, x+width-thickness, y, height/2, thickness, color)
	}
	if segments[segmentStart+2] {
		drawVerticalSegment(img, x+width-thickness, y+height/2, height/2, thickness, color)
	}
	if segments[segmentStart+3] {
		drawHorizontalSegment(img, x, y+height-thickness, width, thickness, color)
	}
	if segments[segmentStart+4] {
		drawVerticalSegment(img, x, y+height/2, height/2, thickness, color)
	}
	if segments[segmentStart+5] {
		drawVerticalSegment(img, x, y, height/2, thickness, color)
	}
	if segments[segmentStart+6] {
		drawHorizontalSegment(img, x, y+height/2-thickness/2, width, thickness, color)
	}
}

func drawHorizontalSegment(img draw.Image, x, y, width, thickness int, color color.RGBA) {
	for dy := 0; dy < thickness; dy++ {
		for dx := 0; dx < width; dx++ {
			if x+dx >= 0 && y+dy >= 0 && x+dx < img.Bounds().Dx() && y+dy < img.Bounds().Dy() {
				img.Set(x+dx, y+dy, color)
			}
		}
	}
}

func drawVerticalSegment(img draw.Image, x, y, height, thickness int, color color.RGBA) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < thickness; dx++ {
			if x+dx >= 0 && y+dy >= 0 && x+dx < img.Bounds().Dx() && y+dy < img.Bounds().Dy() {
				img.Set(x+dx, y+dy, color)
			}
		}
	}
}

func randomColor(r *rand.Rand) color.RGBA {
	return color.RGBA{
		R: uint8(r.Intn(256)),
		G: uint8(r.Intn(256)),
		B: uint8(r.Intn(256)),
		A: 255,
	}
}

func interpolateColor(c1, c2 color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c1.R)*(1-t) + float64(c2.R)*t),
		G: uint8(float64(c1.G)*(1-t) + float64(c2.G)*t),
		B: uint8(float64(c1.B)*(1-t) + float64(c2.B)*t),
		A: 255,
	}
}

func (g *TestImageGenerator) saveJPEG(img image.Image, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	opt := jpeg.Options{Quality: 90}
	return jpeg.Encode(file, img, &opt)
}
