package mcp

import (
	"bytes"
	"context"
	"fmt"
	goimage "image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/rasterizer"

	"photofield/internal/codec"
	jpegcodec "photofield/internal/codec/jpeg"
	webpjack "photofield/internal/codec/webp/jack"
	webpjackdyn "photofield/internal/codec/webp/jack/dynamic"
	webpjacktra "photofield/internal/codec/webp/jack/transpiled"
	"photofield/internal/image"
	"photofield/internal/io"
	"photofield/internal/render"
)

// imagePoolKey is a key for the image pool cache.
type imagePoolKey struct {
	Width  int
	Height int
	Mem    codec.ImageMem
}

// tilePools caches image pools by dimensions to avoid recreating them.
var tilePools sync.Map

// getPool creates or retrieves an image pool for the given dimensions.
func getPool(config *render.Render) *sync.Pool {
	key := imagePoolKey{
		Width:  config.ImageWidth,
		Height: config.ImageHeight,
		Mem:    config.ImageMem,
	}
	if pool, ok := tilePools.Load(key); ok {
		return pool.(*sync.Pool)
	}

	pool := &sync.Pool{}
	switch config.ImageMem {
	case codec.ImageMemNRGBA:
		pool.New = func() interface{} {
			return goimage.NewNRGBA(goimage.Rect(0, 0, config.ImageWidth, config.ImageHeight))
		}
	case codec.ImageMemPaletted:
		pool.New = func() interface{} {
			return goimage.NewPaletted(
				goimage.Rect(0, 0, config.ImageWidth, config.ImageHeight),
				color.Palette{
					color.RGBA{0x00, 0x00, 0x00, 0x00},
					color.RGBA{0xFF, 0xFF, 0xFF, 0xFF},
				},
			)
		}
	default:
		pool.New = func() interface{} {
			return goimage.NewRGBA(goimage.Rect(0, 0, config.ImageWidth, config.ImageHeight))
		}
	}
	stored, _ := tilePools.LoadOrStore(key, pool)
	return stored.(*sync.Pool)
}

// getPoolImage gets an image from the pool and creates a canvas context.
func getPoolImage(config *render.Render) (draw.Image, *canvas.Context) {
	pool := getPool(config)
	img := pool.Get().(draw.Image)
	r := rasterizer.New(img, 1.0)
	c := canvas.NewContext(r)
	c.SetView(canvas.Identity)
	return img, c
}

// putPoolImage returns an image to the pool.
func putPoolImage(config *render.Render, img draw.Image) {
	pool := getPool(config)
	pool.Put(img)
}

// getPhotoInput contains the parameters for the get_photo MCP tool.
type getPhotoInput struct {
	FileId int `json:"file_id" jsonschema:"The photo file ID to retrieve"`

	// Dimensions - if omitted, defaults to thumbnail (256x256 max)
	TargetW *int `json:"w" jsonschema:"Target width in pixels (1-4096). Omitted = auto thumbnail"`
	TargetH *int `json:"h" jsonschema:"Target height in pixels (1-4096). Omitted = auto thumbnail"`

	// Format - if omitted, defaults to JPEG
	Format *string `json:"format" jsonschema:"Output format: jpeg, png, or webp (default jpeg)"`

	// Cropping - all in original image pixel coordinates
	CropX *int `json:"crop_x" jsonschema:"Crop left edge in original image pixels"`
	CropY *int `json:"crop_y" jsonschema:"Crop top edge in original image pixels"`
	CropW *int `json:"crop_w" jsonschema:"Crop width in original image pixels"`
	CropH *int `json:"crop_h" jsonschema:"Crop height in original image pixels"`
}

// getPhotoMetadataInput contains the parameters for the get_photo_metadata MCP tool.
type getPhotoMetadataInput struct {
	FileId int `json:"file_id" jsonschema:"The photo file ID to retrieve metadata for"`
}

// getPhotoOutput is the empty output type for get_photo — this tool returns only
// the image (as MCP ImageContent), no structured metadata. Metadata is available
// via the separate get_photo_metadata tool.
type getPhotoOutput struct{}

// getPhotoMetadataOutput contains the structured metadata for the get_photo_metadata MCP tool.
type getPhotoMetadataOutput struct {
	Width    int         `json:"width"`    // original image width in pixels
	Height   int         `json:"height"`   // original image height in pixels
	Path         string      `json:"path"`          // original file path
	Video        bool        `json:"video,omitempty"`        // true if the file is a video
	CreatedAt    string      `json:"created_at"`    // file creation time in RFC 3339
	Tags         []SimpleTag `json:"tags,omitempty"`
	Faces        []FaceInfo  `json:"faces,omitempty"`
	Location     string      `json:"location,omitempty"`      // reverse-geocoded location
	LatLng       *LatLng     `json:"latlng,omitempty"`         // GPS coordinates
	PreviewUrl   string      `json:"preview_url"`    // absolute URL to a ~400px wide preview image (for direct markdown embedding: ![name](preview_url))
	OriginalUrl  string      `json:"original_url"`  // absolute URL to the original image (full-resolution variant)
}

// FaceInfo represents detected face data for a photo.
type FaceInfo struct {
	Id          int    `json:"id"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	W           int    `json:"w"`
	H           int    `json:"h"`
	Confidence  int    `json:"confidence"`
	PreviewUrl  string `json:"preview_url,omitempty"` // absolute URL to cropped face preview image
}

// LatLng holds GPS coordinates.
type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// SimpleTag represents a tag with a string ID.
type SimpleTag struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	FileCount int    `json:"file_count"`
}

// getPhotoMetadataHandler handles the get_photo_metadata tool request.
// Returns all photo metadata without the image data — useful for inspecting
// tags, faces, location, thumbnails, and dimensions without downloading the image.
func getPhotoMetadataHandler(imageSource *image.Source, srv *Server) mcp.ToolHandlerFor[getPhotoMetadataInput, getPhotoMetadataOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input getPhotoMetadataInput) (*mcp.CallToolResult, getPhotoMetadataOutput, error) {
		// Ensure we have a valid context - fall back to Background if nil.
		if ctx == nil {
			ctx = context.Background()
		}
			// Get file info to validate existence
		info := imageSource.GetInfo(image.ImageId(input.FileId))
		if info.Width == 0 || info.Height == 0 {
			return nil, getPhotoMetadataOutput{}, fmt.Errorf("file not found: %d", input.FileId)
		}

		// Gather metadata using the same logic as get_photo
		var metadata photoMetadata
		var metaErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					metaErr = fmt.Errorf("internal error reading photo metadata: %v", r)
					fmt.Fprintf(os.Stderr, "get_photo_metadata handler recovered from panic: %v\n%s", r, stackTrace())
				}
			}()
			metadata = gatherPhotoMetadata(ctx, imageSource, input.FileId, info, srv.baseURL.Load().(string), srv.apiPrefix)
		}()
		if metaErr != nil {
			return nil, getPhotoMetadataOutput{}, metaErr
		}

			// Return only structured metadata — no image content block.
		// Leave Content nil so the SDK auto-populates it with JSON text
		// from StructuredContent (required for MCP clients that only read content).
		return nil, getPhotoMetadataOutput{
			PreviewUrl:  metadata.PreviewUrl,
			OriginalUrl: metadata.OriginalUrl,
			Width:  info.Width,
			Height: info.Height,
			Path:        metadata.Path,
			Video:       metadata.Video,
			CreatedAt:   metadata.CreatedAt,
			Tags:        metadata.Tags,
			Faces:       metadata.Faces,
			Location:    metadata.Location,
			LatLng:      metadata.LatLng,
		}, nil
	}
}

// getPhotoHandler handles the get_photo MCP tool request.
func getPhotoHandler(imageSource *image.Source, srv *Server) mcp.ToolHandlerFor[getPhotoInput, getPhotoOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input getPhotoInput) (*mcp.CallToolResult, getPhotoOutput, error) {
		// Ensure we have a valid context - fall back to Background if nil.
		if ctx == nil {
			ctx = context.Background()
		}
		// Get file info to validate existence
		info := imageSource.GetInfo(image.ImageId(input.FileId))
		if info.Width == 0 || info.Height == 0 {
			return nil, getPhotoOutput{}, fmt.Errorf("file not found: %d", input.FileId)
		}

		// Determine target dimensions
		targetW, targetH := input.TargetW, input.TargetH
		if targetW == nil || targetH == nil {
			// Default to small thumbnail: 256x256 max (like thumbnail sources)
			w := 256
			h := 256
			targetW = &w
			targetH = &h
		}

		// Parse and validate format
		formatStr := "jpeg"
		if input.Format != nil && *input.Format != "" {
			formatStr = *input.Format
			switch formatStr {
			case "jpeg", "jpg":
				formatStr = "jpeg"
			case "png":
			case "webp":
			default:
				return nil, getPhotoOutput{}, fmt.Errorf("unsupported format: %s (use jpeg, png, or webp)", *input.Format)
			}
		}

		// Encode image data
		var imageData []byte
		var encodeErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					encodeErr = fmt.Errorf("internal error rendering photo: %v", r)
					fmt.Fprintf(os.Stderr, "get_photo handler recovered from panic: %v\n%s", r, stackTrace())
				}
			}()
			imageData, encodeErr = encodePhoto(ctx, imageSource, image.ImageId(input.FileId), *targetW, *targetH, formatStr,
				input.CropX, input.CropY, input.CropW, input.CropH)
		}()
		if encodeErr != nil {
			return nil, getPhotoOutput{}, encodeErr
		}

		mime := "image/jpeg"
		if formatStr == "png" {
			mime = "image/png"
		} else if formatStr == "webp" {
			mime = "image/webp"
		}

		// Return only the image as MCP ImageContent — no structured metadata.
		// Use get_photo_metadata(file_id) to retrieve tags, faces, dimensions, URLs, etc.
		// The SDK handles base64 encoding for the JSON wire format.
		res := &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.ImageContent{
					Data:     imageData,
					MIMEType: mime,
				},
			},
		}
		return res, getPhotoOutput{}, nil
	}
}

// encodePhoto renders and encodes a photo to the specified format.
func encodePhoto(ctx context.Context, source *image.Source, fileId image.ImageId, targetW, targetH int, format string,
	cropX, cropY, cropW, cropH *int) ([]byte, error) {

	// Get file info
	info := source.GetInfo(fileId)
	if info.Width == 0 || info.Height == 0 {
		return nil, fmt.Errorf("file not found: %d", fileId)
	}

	// Validate dimensions
	if targetW < 1 || targetW > 4096 || targetH < 1 || targetH > 4096 {
		return nil, fmt.Errorf("dimensions %dx%d out of range (1-4096)", targetW, targetH)
	}

	// Validate crop bounds
	if cropW != nil && cropH != nil && *cropW > 0 && *cropH > 0 {
		cx := 0
		cy := 0
		if cropX != nil {
			cx = *cropX
		}
		if cropY != nil {
			cy = *cropY
		}
		if cx < 0 || cy < 0 || cx+*cropW > info.Width || cy+*cropH > info.Height {
			return nil, fmt.Errorf("crop bounds (%d,%d)+(%dx%d) exceed image size (%dx%d)",
				cx, cy, *cropW, *cropH, info.Width, info.Height)
		}
	}

	// Create render config (similar to defaultSceneConfig.Render)
	rn := render.Render{
		TileSize:          256,
		ImageWidth:        targetW,
		ImageHeight:       targetH,
		MaxSolidPixelArea: 0, // Force full render, no solid color optimization
		BackgroundColor:   color.RGBA{0, 0, 0, 0},
		CoverFit:          true,
		ImageMem:          codec.ImageMemRGBA,
		QualityPreset:     render.QualityPresetFast,
	}

	// Get pooled image and canvas context
	img, c := getPoolImage(&rn)
	defer putPoolImage(&rn, img)

	rn.CanvasImage = img

	// Reset view and setup coordinates
	c.ResetView()
	c.SetView(canvas.Identity.Translate(0, float64(targetH)))

	// Draw background
	draw.Draw(img, img.Bounds(), &goimage.Uniform{rn.BackgroundColor}, goimage.Point{}, draw.Src)

	// Setup photo with full content area bounds (no border)
	photo := &render.Photo{
		Id: fileId,
	}
	photo.Sprite.Rect = render.Rect{
		X: 0,
		Y: 0,
		W: float64(targetW),
		H: float64(targetH),
	}

	// Build optional crop rect
	var crop render.Rect
	if cropW != nil && cropH != nil && *cropW > 0 && *cropH > 0 {
		cx := 0
		cy := 0
		if cropX != nil {
			cx = *cropX
		}
		if cropY != nil {
			cy = *cropY
		}
		crop = render.Rect{
			X: float64(cx),
			Y: float64(cy),
			W: float64(*cropW),
			H: float64(*cropH),
		}
	}

	// Draw photo using existing rendering logic
	photo.Draw(ctx, &rn, nil, c, render.Scales{Tile: 1.0}, source, false, crop)

	// Encode to requested format
	var buf bytes.Buffer
	quality := 80
	if rn.QualityPreset == render.QualityPresetHigh {
		quality = 100
	}

	switch format {
	case "jpeg":
		if err := jpegcodec.Encode(&buf, img, quality); err != nil {
			return nil, fmt.Errorf("error encoding JPEG: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("error encoding PNG: %w", err)
		}
	case "webp":
		// WebP encoding - try the available encoders
		encoders := []func(w *bytes.Buffer, img goimage.Image, quality int) error{
			func(w *bytes.Buffer, img goimage.Image, quality int) error {
				return webpjack.Encode(w, img, quality)
			},
			func(w *bytes.Buffer, img goimage.Image, quality int) error {
				return webpjackdyn.Encode(w, img, quality)
			},
			func(w *bytes.Buffer, img goimage.Image, quality int) error {
				return webpjacktra.Encode(w, img, quality)
			},
		}
		encoded := false
		for _, enc := range encoders {
			err := enc(&buf, img, quality)
			if err == nil {
				encoded = true
				break
			}
			buf.Reset()
		}
		if !encoded {
			// Fallback to JPEG if no WebP encoder works
			if err := jpegcodec.Encode(&buf, img, quality); err != nil {
				return nil, fmt.Errorf("error encoding JPEG (WebP fallback): %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return buf.Bytes(), nil
}

// photoMetadata holds all metadata fields for a photo.
type photoMetadata struct {
	Path        string
	Video       bool
	CreatedAt   string
	PreviewUrl  string
	OriginalUrl string
	Tags        []SimpleTag
	Faces       []FaceInfo
	Location    string
	LatLng      *LatLng
}

// fileURL builds an absolute URL to a file endpoint, handling an empty or root apiPrefix.
func fileURL(serverBaseURL, apiPrefix, path string) string {
	if apiPrefix == "" || apiPrefix == "/" {
		return serverBaseURL + path
	}
	return serverBaseURL + apiPrefix + path
}

// gatherPhotoMetadata collects all metadata for a photo by file ID.
// serverBaseURL is the absolute API base URL (e.g. "http://localhost:8080").
// apiPrefix is the HTTP route prefix for file endpoints (e.g. "/api" or "").
func gatherPhotoMetadata(ctx context.Context, source *image.Source, fileId int, info image.Info, serverBaseURL, apiPrefix string) photoMetadata {
	originalPath, _ := source.GetImagePath(image.ImageId(fileId))
	location := ""
	var latlng *LatLng
	if image.IsValidLatLng(info.LatLng) {
		latlng = &LatLng{
			Lat: info.LatLng.Lat.Degrees(),
			Lng: info.LatLng.Lng.Degrees(),
		}
		if source.Geo != nil && source.Geo.Available() {
			location, _ = source.Geo.ReverseGeocode(ctx, info.LatLng)
		}
	}

	isVideo := source.IsSupportedVideo(originalPath)
	filename := filepath.Base(originalPath)
	previewFilename := strings.TrimSuffix(filename, filepath.Ext(filename)) + "_preview.jpg"

	// Build preview URL: use /api/files/{id}/previews/{filename} with ~400px width for direct markdown embedding
	var previewUrl string
	if originalPath != "" {
		previewUrl = fileURL(serverBaseURL, apiPrefix, "/files/"+fmt.Sprintf("%d", fileId)+"/previews/"+previewFilename+"?w=400")
	}

	// Build original image URL: use the 'original' variant (full-resolution source copy)
	var originalUrl string
	for _, s := range source.Sources {
		if s.Name() != "original" {
			continue
		}
		if !s.Exists(ctx, io.ImageId(fileId), originalPath) {
			continue
		}
		originalUrl = fileURL(serverBaseURL, apiPrefix, "/files/"+fmt.Sprintf("%d", fileId)+"/variants/"+s.Name()+"/"+filename)
		break
	}

	// Gather tags
	tags := make([]SimpleTag, 0)
	for t := range source.ListImageTags(image.ImageId(fileId)) {
		tags = append(tags, SimpleTag{
			Id:        t.Name,
			Name:      t.Name,
			FileCount: t.FileCount,
		})
	}

	// Gather face detections
	faceInfos := source.GetFacesByFileId(image.ImageId(fileId))
	faces := make([]FaceInfo, 0, len(faceInfos))
	for _, f := range faceInfos {
		// Build face preview URL: crop a square around the face
		faceCropSize := f.W
		if f.H > faceCropSize {
			faceCropSize = f.H
		}
		// Center the crop on the face
		cropX := f.X + (f.W-faceCropSize)/2
		if cropX < 0 {
			cropX = 0
		}
		cropY := f.Y + (f.H-faceCropSize)/2
		if cropY < 0 {
			cropY = 0
		}
		faceFilename := fmt.Sprintf("face_%d.jpg", f.Id)
		faces = append(faces, FaceInfo{
			Id:          f.Id,
			X:           f.X,
			Y:           f.Y,
			W:           f.W,
			H:           f.H,
			Confidence:  f.Confidence,
			PreviewUrl:  fileURL(serverBaseURL, apiPrefix, "/files/"+fmt.Sprintf("%d", fileId)+"/previews/"+faceFilename+"?w=200&h=200&crop_x="+fmt.Sprintf("%d", cropX)+"&crop_y="+fmt.Sprintf("%d", cropY)+"&crop_w="+fmt.Sprintf("%d", faceCropSize)+"&crop_h="+fmt.Sprintf("%d", faceCropSize)),
		})
	}

	if originalUrl == "" {
		originalUrl = fileURL(serverBaseURL, apiPrefix, "/files/"+fmt.Sprintf("%d", fileId)+"/variants/"+filename)
	}

	return photoMetadata{
		Path:        originalPath,
		Video:       isVideo,
		CreatedAt:   info.DateTime.Format("2006-01-02T15:04:05Z07:00"),
		PreviewUrl:  previewUrl,
		OriginalUrl: originalUrl,
		Tags:        tags,
		Faces:       faces,
		Location:    location,
		LatLng:      latlng,
	}
}

// stackTrace returns a formatted stack trace for logging panics.
func stackTrace() string {
	const maxStack = 32
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
