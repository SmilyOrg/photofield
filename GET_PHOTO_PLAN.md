# Plan: MCP `get_photo` tool

## Goal
Add an MCP tool that returns a photo (image data) by its ID, defaulting to a small/medium preview thumbnail. Users can optionally customize dimensions, format, and crop — mirroring the existing `GET /files/{id}/previews/{filename}` API parameters.

---

## 1. New file: `internal/mcp/photo.go`

Contains the input/output types and handler for the `get_photo` tool.

### `getPhotoInput` struct

```go
type getPhotoInput struct {
    CollectionId string  `json:"collection_id" jsonschema:"The collection ID containing the photo"`
    FileId       int     `json:"file_id" jsonschema:"The photo file ID to retrieve"`

    // Dimensions — if omitted, defaults to thumbnail (256×256 max)
    TargetW *int `json:"w" jsonschema:"Target width in pixels (1-4096). Omitted = auto thumbnail"`
    TargetH *int `json:"h" jsonschema:"Target height in pixels (1-4096). Omitted = auto thumbnail"`

    // Format — if omitted, defaults to JPEG
    Format *string `json:"format" jsonschema:"Output format: jpeg, png, or webp (default jpeg)"`

    // Cropping — all in original image pixel coordinates
    CropX *int `json:"crop_x" jsonschema:"Crop left edge in original image pixels"`
    CropY *int `json:"crop_y" jsonschema:"Crop top edge in original image pixels"`
    CropW *int `json:"crop_w" jsonschema:"Crop width in original image pixels"`
    CropH *int `json:"crop_h" jsonschema:"Crop height in original image pixels"`
}
```

### `getPhotoOutput` struct

```go
type getPhotoOutput struct {
    Data      string `json:"data"`        // base64-encoded image bytes
    Format    string `json:"format"`       // "jpeg", "png", or "webp"
    Width     int    `json:"width"`        // output width in pixels
    Height    int    `json:"height"`       // output height in pixels
    Media     string `json:"media"`        // MIME type (e.g. "image/jpeg")
}
```

> **Rationale for base64**: MCP `CallToolResult` supports `ContentText` and `ContentImage` with `data` + `mimeType`. The `go-sdk` `mcp` package's `CallToolResult` type accepts an `image` content block with base64 data. This avoids needing to stream raw bytes through the MCP protocol.

### `getPhotoHandler` function

```go
func getPhotoHandler(collections *[]collection.Collection, imageSource *image.Source) mcp.ToolHandlerFor[getPhotoInput, getPhotoOutput] {
    return func(ctx context.Context, _ *mcp.CallToolRequest, input getPhotoInput) (*mcp.CallToolResult, getPhotoOutput, error) {
        defer func() {
            if r := recover(); r != nil {
                fmt.Fprintln(os.Stderr, "get_photo handler recovered from panic:", r)
            }
        }()

        // 1. Resolve collection (optional — file_id is absolute in the DB)
        //    Skip collection validation since file IDs are global within a source.
        //    Just verify the file exists in the DB.

        // 2. Get file info to validate existence
        info := imageSource.GetInfo(image.ImageId(input.FileId))
        if info.Width == 0 || info.Height == 0 {
            return nil, getPhotoOutput{}, fmt.Errorf("file not found: %d", input.FileId)
        }

        // 3. Determine target dimensions
        targetW, targetH := input.TargetW, input.TargetH
        if targetW == nil || targetH == nil {
            // Default to small thumbnail: 256×256 max (like thumbnail sources)
            targetW = intPtr(256)
            targetH = intPtr(256)
        }

        // 4. Parse format
        formatStr := "jpeg"
        if input.Format != nil && *input.Format != "" {
            formatStr = strings.ToLower(*input.Format)
        }
        // Validate format
        switch formatStr {
        case "jpeg", "jpg":
            formatStr = "jpeg"
        case "png":
        case "webp":
        default:
            return nil, getPhotoOutput{}, fmt.Errorf("unsupported format: %s (use jpeg, png, or webp)", formatStr)
        }

        // 5. Encode image data using the preview render pipeline
        //    Reuse the same approach as GetFilesIdPreviewsFilename in main.go:
        //    - Create render config (defaultSceneConfig.Render)
        //    - Set ImageWidth, ImageHeight, MaxSolidPixelArea=0
        //    - Render photo to canvas
        //    - Encode to requested format
        //    - Base64-encode the result
        imageData, err := encodePhoto(imageSource, input.FileId, targetW, targetH, formatStr,
            input.CropX, input.CropY, input.CropW, input.CropH)
        if err != nil {
            return nil, getPhotoOutput{}, err
        }

        mime := "image/jpeg"
        if formatStr == "png" {
            mime = "image/png"
        } else if formatStr == "webp" {
            mime = "image/webp"
        }

        return nil, getPhotoOutput{
            Data:    base64.StdEncoding.EncodeToString(imageData),
            Format:  formatStr,
            Width:   *targetW,
            Height:  *targetH,
            Media:   mime,
        }, nil
    }
}
```

### `encodePhoto` helper

This mirrors the preview endpoint logic from `main.go:GetFilesIdPreviewsFilename`, extracted into a reusable function:

```go
func encodePhoto(source *image.Source, fileId int, targetW, targetH *int, format string,
    cropX, cropY, cropW, cropH *int) ([]byte, error) {

    photoId := image.ImageId(fileId)

    // Get info
    info := source.GetInfo(photoId)
    if info.Width == 0 || info.Height == 0 {
        return nil, fmt.Errorf("file not found: %d", fileId)
    }

    // Build crop rect (if specified)
    var crop *render.Rect
    if cropW != nil && cropH != nil && *cropW > 0 && *cropH > 0 {
        cx, cy := 0, 0
        if cropX != nil {
            cx = *cropX
        }
        if cropY != nil {
            cy = *cropY
        }
        c := render.Rect{
            X: float64(cx),
            Y: float64(cy),
            W: float64(*cropW),
            H: float64(*cropH),
        }
        crop = &c
    }

    // Setup render config
    rn := defaultSceneConfig.Render
    rn.ImageWidth = *targetW
    rn.ImageHeight = *targetH
    rn.MaxSolidPixelArea = 0
    rn.BackgroundColor = color.RGBA{0, 0, 0, 0}
    rn.CoverFit = true

    // Get pooled image and canvas context
    img, c := getPoolImage(&rn)
    defer putPoolImage(&rn, img)
    rn.CanvasImage = img

    // Render (same pipeline as preview API, no border)
    // ... draw background, photo sprite, etc. ...

    // Encode to format
    var buf bytes.Buffer
    switch format {
    case "jpeg":
        // encodeJPEG(img, &buf, 85)
    case "png":
        // png.Encode(&buf, img)
    case "webp":
        // encodeWebP(img, &buf)
    }

    return buf.Bytes(), nil
}
```

> **Note**: The `defaultSceneConfig` and `getPoolImage`/`putPoolImage` functions are already in `main.go`. The `encodePhoto` function will extract the rendering logic from `GetFilesIdPreviewsFilename` into a shared helper, or simply inline the preview handler code here. If we want to avoid code duplication, consider moving the preview rendering into a new file like `internal/render/preview.go`.

---

## 2. Register the tool in `internal/mcp/mcp.go`

Add a fourth tool registration in `New()`:

```go
mcp.AddTool(s, &mcp.Tool{
    Name:        "get_photo",
    Description: "Return a photo by ID as a base64-encoded image (JPEG/PNG/WebP). Defaults to a 256×256 thumbnail. Supports custom dimensions and cropping.",
}, getPhotoHandler(collections, imageSource))
```



## File Changes Summary

| File | Action | What |
|------|--------|------|
| `internal/mcp/photo.go` | **New** | `getPhotoInput`, `getPhotoOutput`, `getPhotoHandler`, `encodePhoto` |
| `internal/mcp/mcp.go` | **Edit** | Add `get_photo` tool registration |



---

## Default Behavior (when no params provided)

| Parameter | Default | Rationale |
|-----------|---------|-----------|
| `w` / `h` | 256 / 256 | Matches thumbnail source sizes (djpeg generator uses 256px); small enough for fast loading, large enough to be useful |
| `format` | `jpeg` | Smallest file size for photos; universally supported |
| `crop_*` | none (full image) | No crop applied by default |


When `w` or `h` is specified but not the other, the aspect ratio of the original image (or cropped region) is preserved.

---

## MCP Content Image Format

The `go-sdk` MCP library supports returning image content directly:

```go
result := mcp.NewCallToolResult(
    mcp.WithContentImage(mcp.ImageContent{
        Data:      base64Data,
        MediaType: "image/jpeg",
    }),
)
return result, output, nil
```

This lets LLM clients display the photo inline rather than just showing base64 text.

---

## Testing approach

1. **MCP `get_photo` with no params** → verify 256×256 JPEG thumbnail returned
2. **MCP `get_photo` with `w=512, h=384`** → verify correct dimensions and aspect ratio
3. **MCP `get_photo` with `format=png`** → verify PNG-encoded data
4. **MCP `get_photo` with `crop_x=100, crop_y=50, crop_w=200, crop_h=200`** → verify cropped region
5. **MCP `get_photo` with non-existent file_id** → verify 404-style error
7. **Compare MCP output vs API `/files/{id}/previews`** → pixel-identical output
