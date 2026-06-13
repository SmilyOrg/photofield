# Plan: MCP `search_photos` tool + API `GET /collections/{id}/files`

## Goal
Add an MCP tool and API endpoint that lets users search a collection's photos using natural language or structured queries. The core logic mirrors what `SceneSource.loadScene` does (parse query → embed → query DB → return results), but skips the rendering/layout layer. The API uses query params (`search`, `sort`, `limit`) following the `SceneParams` pattern from `GET /scenes`.

---

## 1. New file: `internal/collection/search.go`

### `SearchResult` type
```go
type SearchResult struct {
    Id         int32   `json:"id"`
    FileName   string  `json:"file_name"`
    DateTime   string  `json:"datetime,omitempty"`
    Width      int     `json:"width,omitempty"`
    Height     int     `json:"height,omitempty"`
    Color      string  `json:"color,omitempty"`      // hex color
    Location   string  `json:"location,omitempty"`   // reverse-geocoded, if geo available
    Similarity float32 `json:"similarity,omitempty"` // cosine similarity score
    Tags       []string `json:"tags,omitempty"`     // tags on this photo
}
```

### `Collection.Search` method
```go
type SearchOptions struct {
    QueryStr string
    Sort     SortType  // e.g. "date", "similarity"
    Limit    int
    Offset   int
}

type SortType string

func (collection *Collection) Search(
    ctx context.Context,
    source *image.Source,
    opts SearchOptions,
) ([]SearchResult, []search.Token, []search.FieldError, error)
```

Steps (mirrors `sceneSource.loadScene`):

1. **Parse query** — `search.Parse(queryStr)` → `*search.Query`
2. **Validate/extract expression** — `query.Expression()` → `search.Expression`
   - Collect tokens: `query.Tokens()`
   - Collect errors: `expression.Errors`
   - Extract text, created range, filters, tags, filenames, img, face
3. **Resolve embeddings** (same flow as scene loading):
   - If `expression.Image.Present` → `source.GetImageEmbedding(image.ImageId(val))`
   - If `expression.Face.Present` → `source.GetFaceEmbedding(val)`
   - If text is non-empty and no image/face embed yet → `source.Clip.EmbedText(text)`
   - If embedding fails, set error
4. **Query DB** — `collection.GetInfos(source, image.ListOptions{…})`:
   - `OrderBy`: determined by `opts.Sort` + whether any embedding is present
   - `Limit`: `opts.Limit` (default 50)
   - `Offset`: `opts.Offset` (default 0)
   - `Expression`: the parsed expression
   - `ImageEmbedding` / `FaceEmbedding`: resolved above
   - `Extensions`: nil (no filter)
5. **Collect results** from channel:
   - For each `SourcedInfo`, build a `SearchResult`:
     - `Id`: `info.Id`
     - `FileName`: parse `source.GetImagePath(info.Id)` with `filepath.Base()`
     - `DateTime`: `info.DateTime` (formatted as RFC3339)
     - `Width`/`Height`: `info.Width`/`info.Height`
     - `Color`: `info.Color` as hex string
     - `Location`: reverse-geocoded (if geo available)
     - `Similarity`: `info.Similarity`
     - `Tags`: call `source.ListImageTags(info.Id)`, collect names
   - Stop after `limit` results
6. **Return** `[]SearchResult`, tokens, errors, and any embedding error.

### Helper: reverse-geocode a single photo
```go
func reverseGeocode(ctx context.Context, source *image.Source, latLng s2.LatLng) string
```

---

## 2. Thin out `internal/mcp/mcp.go`

Add a third tool registration:
```go
mcp.AddTool(s, &mcp.Tool{
    Name:        "search_photos",
    Description: "Search a collection's photos by text, image reference, face reference, or structured qualifiers. Returns metadata and similarity scores.",
}, searchPhotosHandler(collections, imageSource))
```

Handler delegates to `coll.Search()`.

---

## 3. API endpoint: `GET /collections/{id}/files`

Follows the `SceneParams` pattern: `search` and `sort` are query params (not body), with `limit` for pagination.

### `api.yaml`
```yaml
paths:
  /collections/{id}/files:
    get:
      description: Search a collection's photos by text or structured query.
      tags: ["Source"]
      parameters:
        - name: id
          in: path
          required: true
          schema:
            $ref: "#/components/schemas/CollectionId"

        - name: search
          in: query
          description: Natural language or structured search query
          schema:
            $ref: "#/components/schemas/Search"

        - name: sort
          in: query
          description: Sort order. Prefix with `-` for descending or `+` for ascending, e.g. `-date` (newest first), `+date` (oldest first), `-similarity` (best matches first). Multiple values allowed.
          schema:
            $ref: "#/components/schemas/Sort"

        - name: limit
          in: query
          description: Maximum number of results
          schema:
            $ref: "#/components/schemas/Limit"

      responses:
        "200":
          description: Search results
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/FileList"

components:
  schemas:
    FileList:
      type: object
      properties:
        items:
          type: array
          items:
            $ref: "#/components/schemas/FileInfo"

    FileInfo:
      type: object
      properties:
        id: { type: integer }
        file_name: { type: string }
        datetime: { type: string, format: date-time }
        width: { type: integer }
        height: { type: integer }
        color: { type: string }
        location: { type: string }
        similarity: { type: number }
        tags:
          type: array
          items: { type: string }
```

### `main.go` handler
```go
func (*Api) GetCollectionsIdFiles(w http.ResponseWriter, r *http.Request, id openapi.CollectionsIdFilesParams) {
    coll := getCollectionById(string(id))
    if coll == nil {
        problem(w, r, http.StatusBadRequest, "Collection not found")
        return
    }

    limit := 50
    if id.Limit != nil {
        limit = int(*id.Limit)
    }

    opts := collection.SearchOptions{
        QueryStr: string(id.Search),
        Sort:     collection.SortType(string(id.Sort)),
        Limit:    limit,
    }

    items, tokens, errs, err := coll.Search(r.Context(), imageSource, opts)
    if err != nil {
        problem(w, r, http.StatusInternalServerError, err.Error())
        return
    }

    respond(w, r, http.StatusOK, openapi.FileList{
        Items: &items,
    })
}
```

---

## File Changes Summary

| File | Action | What |
|------|--------|------|
| `internal/collection/search.go` | **New** | `SearchResult`, `SearchOptions`, `SortType`, `Search` method, `reverseGeocode` helper |
| `internal/mcp/mcp.go` | **Edit** | Add `search_photos` tool registration + handler |
| `api.yaml` | **Edit** | Add `GET /collections/{id}/files` with `search`, `sort`, `limit` query params; add `FileInfo`, `FileList` schemas |
| `internal/openapi/api.gen.go` | **Regenerate** | `go generate ./...` |
| `main.go` | **Edit** | Add `GetCollectionsIdFiles` handler |

---

## Key Design Decisions

1. **Reuse `collection.GetInfos()`** — same DB query path as events and scenes; no duplication
2. **Embedding logic matches scene loading** — text→clip embed, image reference→DB embed, face reference→DB embed, all with error handling
3. **Query params follow `SceneParams` pattern** — `search` and `sort` as query params (not body), `limit` for pagination
4. **Default limit 50** — prevents runaway responses; MCP tool caps at 50, API uses `Limit` type (integer)
5. **Location reverse-geocoding** — same as events tool (15 min / 1 km gap), but applied per-result so the user sees locations for all matches
6. **No layout/rendering** — skips the entire `layout.Layout*` machinery; just returns metadata + similarity scores
7. **Tokens returned** — so callers can see how the query was parsed (matching the existing API search query endpoint)

---

## Search query syntax (from existing `search` package)

| Query | Meaning |
|-------|---------|
| `sunset beach` | Text search by cosine similarity |
| `created:2024-06` | Photos from June 2024 |
| `tag:vacation` | Photos tagged "vacation" |
| `filename:IMG_` | Files matching glob |
| `img:123` | Similar to photo with ID 123 |
| `face:456` | Similar to face with ID 456 |
| `t:0.3` | Similarity threshold (default 0.262) |
| `filter:knn` | Use knn index directly |
| `k:10` | Return top 10 results |
| `NOT beach` | Exclude "beach" |

---

## Testing approach

1. MCP `search_photos` tool with text query → verify results
2. API `GET /collections/{id}/files` with same query → verify identical results
3. Test `sort=date`, `sort=date_asc`, `sort=similarity` → verify ordering
4. Test `limit` pagination → verify result counts
5. Edge cases: empty query, invalid query, non-existent collection, no AI service, geo disabled
