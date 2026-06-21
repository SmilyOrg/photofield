# PR #190 — Fix Checklist

Issues selected for fixing (validated by Copilot review + manual investigation).

## High Priority

### [x] 1. `tools/agent.sh:91` — `server start` ignores `AGT_PORT`/`AGT_DATA_DIR` ✅ Committed

**Problem:** `server_start()` launches the server with `nohup "$BIN"` but never passes `PHOTOFIELD_ADDRESS` or `PHOTOFIELD_DATA_DIR` env vars. The script defines `PORT` and `DATA_DIR` locals, but the server reads different names (`PHOTOFIELD_ADDRESS` / `PHOTOFIELD_DATA_DIR`).

**Fix:** Export the correct env vars before launching the server in `server_start()`:
```bash
export PHOTOFIELD_ADDRESS=":$(echo "$PORT" | sed 's/.*://')"
export PHOTOFIELD_DATA_DIR="$DATA_DIR"
nohup "$BIN" > /tmp/photofield-agent.log 2>&1 &
```

Remove any AGT_* env vars that are not needed and update the docs

---

### [x] 2. `main.go:1749` — `parsePreviewDimensions` 4096px cap removed (DoS vulnerability) ✅ Committed

**Problem:** Upper bound check (`w > 4096` / `h > 4096`) was accidentally removed in commit `f03b99c`. An attacker can request unbounded dimensions, causing memory exhaustion (40GB+ for 100k×100k).

**Fix:** Clamp input params to 4096 before computing the final size.

---

### [x] 3. `main.go:2237` — Top-level `recover()` suppresses panic stack trace ✅ Committed

**Problem:** The top-level `defer recover()` calls `os.Exit(1)` after only printing `PANIC: %v\n`, losing the full stack trace. Startup panics become nearly impossible to debug.

**Fix:** Remove the top-level recover entirely. Let Go's default panic handler print the full stack trace and crash:
```go
// Remove these lines from main():
// defer func() {
//     if r := recover(); r != nil {
//         fmt.Fprintf(os.Stderr, "PANIC: %v\n", r)
//         os.Exit(1)
//     }
// }()
```

---

### [x] 4. `main.go:2463` — `/health` endpoint registered under `apiPrefix` ✅ Committed

**Problem:** PR description says a top-level `/health` endpoint is added, but it's registered inside `r.Route(apiPrefix, ...)`, making it reachable at `/api/health` (default) instead of `/health`.

**Fix:** Update documentation (not code) — `/api/health` is the correct path given the `apiPrefix` design. Update:
- `docs/mcp-server.md` — document that `/health` is at `/api/health`
- PR body — clarify the path
- Possibly update the commit message or add a note

---

## Medium Priority

### [x] 5. `internal/mcp/mcp.go:216` — `New()` returns different Server instance ✅ Committed

**Problem:** `New()` creates a local `srv`, captures it in handler closures, but returns a different `*Server` instance. Handlers work because they close over the local `srv`, and the returned Server only has `handler` set — sufficient for `Handler()` to work.

**Fix:** Consolidate to a single instance. Remove the local `srv` variable and use the returned one:
```go
func New(...) (*Server, error) {
    sdkSrv := mcp.NewServer(...)
    srv := &Server{srv: sdkSrv}  // single instance
    // ... handlers capture srv ...
    return &Server{
        srv:     sdkSrv,
        handler: wrappedHandler,
        baseURL: atomic.Value{},
        apiPrefix: apiPrefix,
    }, nil
}
```
Or simpler: just return `srv` after setting its `handler` and `baseURL` fields.

---

### [x] 6. `internal/mcp/mcp.go:109` — JSON schema uses `[3]string{"null", "string"}` ✅ Committed

**Problem:** `[3]string{"null", "string"}` produces a 3-element array `["null","string",""]` (empty string is the zero value). The `""` is not a valid JSON Schema type.

**Fix:** Change to `[2]string{"null", "string"}`:
```go
"sort": map[string]any{
    "type": [2]string{"null", "string"},
    "description": "Sort order...",
},
```

---

### [x] 7. `internal/mcp/mcp.go:269` — Events returns empty result for unknown collection ✅ Committed

**Problem:** When `collection_id` is invalid, the events handler returns `{ "events": [] }` with no error — indistinguishable from an empty collection.

**Fix:** Return an error instead:
```go
if coll == nil {
    return nil, eventsOutput{}, fmt.Errorf("collection not found: %s", input.CollectionId)
}
```

---

### [x] 8. `internal/mcp/mcp.go:310` — Search returns empty result for unknown collection ✅ Committed

**Problem:** Same as #7 — the search handler silently returns empty results for invalid collection IDs.

**Fix:** Return an error:
```go
if coll == nil {
    return nil, searchPhotosOutput{}, fmt.Errorf("collection not found: %s", input.CollectionId)
}
```

---

### [x] 9. `internal/mcp/photo.go:359` — `encodePhoto` panics with partial crop params ✅ Committed

**Problem:** The crop rect building block dereferences `*cropX` and `*cropY` without nil checks. Sending `{crop_w: 100, crop_h: 100}` without `crop_x`/`crop_y` triggers a panic.

**Fix:** Add nil defaults in the rect building block (mirror the validation block's approach):
```go
var crop render.Rect
if cropW != nil && cropH != nil && *cropW > 0 && *cropH > 0 {
    cx := 0
    cy := 0
    if cropX != nil { cx = *cropX }
    if cropY != nil { cy = *cropY }
    crop = render.Rect{
        X: float64(cx),
        Y: float64(cy),
        W: float64(*cropW),
        H: float64(*cropH),
    }
}
```

---

### [x] 10. `internal/mcp/photo.go:454` — `gatherPhotoMetadata` doesn't check Geo is non-nil ✅ Committed

**Problem:** `source.Geo` is a `*geo.Geo` pointer that can be nil when geo is disabled. Calling `source.Geo.ReverseGeocode()` panics.

**Fix:** Add a nil check before calling `ReverseGeocode`:
```go
if image.IsValidLatLng(info.LatLng) {
    latlng = &LatLng{
        Lat: info.LatLng.Lat.Degrees(),
        Lng: info.LatLng.Lng.Degrees(),
    }
    if source.Geo != nil && source.Geo.Available() {
        location, _ = source.Geo.ReverseGeocode(ctx, info.LatLng)
    }
}
```

---

### [x] 11. `internal/mcp/photo.go:514` — Face PreviewUrl 404 ✅ Committed

**Problem:** Face `PreviewUrl` uses `/files/{id}/face.jpg?...`, but the OpenAPI routes only expose `/files/{id}/original/...`, `/files/{id}/variants/...`, and `/files/{id}/previews/...`. All face preview URLs will 404.

**Fix:** Use the `/previews/` route with a face-specific filename, consistent with how `PreviewUrl` is built for photos (line ~462):
```go
// Build a face-specific preview filename
faceFilename := fmt.Sprintf("face_%d.jpg", f.Id)
faces = append(faces, FaceInfo{
    ...
    PreviewUrl: fileURL(serverBaseURL, apiPrefix, "/files/"+fmt.Sprintf("%d", fileId)+"/previews/"+faceFilename+"?w=200&h=200"),
})
```

---

### [x] 12. `internal/mcp/photo.go:189` — `panicked` flag is dead code in `get_photo_metadata` ✅ Committed

**Problem:** The deferred `recover()` sets `panicked = r`, but the `if panicked != nil` check after `gatherPhotoMetadata` is unreachable dead code. When `gatherPhotoMetadata` panics, Go's defer machinery returns immediately.

**Fix:** Restructure the panic handling. Either:
- **Option A:** Remove the `panicked` flag entirely and handle panics inside `gatherPhotoMetadata` by catching them before they escape the handler.
- **Option B:** Move the `if panicked != nil` check *before* the `return nil, ...` statement in the same defer scope, but this requires restructuring so the handler returns via the recover path rather than normal flow.

Best approach: Wrap `gatherPhotoMetadata` in its own inline func with defer/recover, and return early if it panics:
```go
var meta photoMetadata
var metaErr error
func() {
    defer func() {
        if r := recover(); r != nil {
            metaErr = fmt.Errorf("internal error reading photo metadata: %v", r)
            fmt.Fprintf(os.Stderr, "get_photo_metadata handler recovered from panic: %v\n%s", r, stackTrace())
        }
    }()
    meta = gatherPhotoMetadata(ctx, imageSource, input.FileId, info, srv.baseURL.Load().(string), srv.apiPrefix)
}()
if metaErr != nil {
    return nil, getPhotoMetadataOutput{}, metaErr
}
```

---

### [x] 13. `internal/mcp/photo.go:223` — Same dead code in `get_photo` ✅ Committed

**Problem:** Identical broken `panicked` flag pattern in `getPhotoHandler`. If `encodePhoto` panics, the `if panicked != nil` check is unreachable.

**Fix:** Same restructuring as #12 — wrap `encodePhoto` in an inline func with defer/recover:
```go
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
```

This eliminates the dead code pattern entirely.

---

## Notes

- **Comment #8** (no MCP test coverage) — left as-is per instructions
- **Comments #3/#4** (400 vs 404 for unknown collections) — left as-is per instructions
- **Comment #16** (fallback originalUrl wrong path) — left as-is per instructions
- **Comment #19** (lastLocTime not reset) — left as-is per instructions
