// Package mcp provides a Model Context Protocol (MCP) server for the
// photofield application, mounted onto the chi HTTP router.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"photofield/internal/collection"
	"photofield/internal/image"
)

// Server holds the MCP server instance and its chi-mountable HTTP handler.
type Server struct {
	srv           *mcp.Server
	handler       http.Handler
	serverBaseURL string // e.g. "http://localhost:8080" — used to build absolute image URLs
}

// New creates a new MCP server for photofield with the given data sources
// and registers all available tools. The serverBaseURL parameter is the
// absolute URL at which the photofield API is accessible (e.g. "http://localhost:8080").
// This is used to construct absolute image URLs for embedding in markdown etc.
// Callers should mount handler() on a chi router, e.g.:
//
//	r.Mount("/mcp", s.handler())
func New(collections *[]collection.Collection, imageSource *image.Source, serverBaseURL string) (*Server, error) {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "photofield",
		Version: "dev",
	}, nil)

	// Handler closure captures collections and imageSource.
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_collections",
		Description: "List all photo collections available in the library with their current indexed status. " +
			"Use this first to discover which collections exist, their IDs, how many photos are indexed, " +
			"and when indexing last occurred. The collection ID from the response is required for all other " +
			"tools (events, search_photos, get_photo). This tool has no input parameters — call it with an empty object {}. " +
			"Returns indexed_count (how many photos have been processed) and indexed_at (timestamp of last indexing). " +
			"If indexed_count is 0 or indexed_at is missing, the collection has not been indexed yet.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, listCollections(collections, imageSource))

	mcp.AddTool(s, &mcp.Tool{
		Name: "events",
		Description: "Split a collection's photos into chronological events based on time gaps. Photos on different " +
			"calendar days, or more than 1 hour apart (within the same day), are placed in separate events. Returns " +
			"metadata summaries only (photo count, date ranges, number of distinct locations, location names) — NOT " +
			"the photo images themselves. Uses reverse-geocoded location names for photos that are more than 1 km " +
			"apart AND more than 15 minutes apart (to avoid excessive geocoding API calls). Best used after " +
			"list_collections to pick a collection_id, then before search_photos to get high-level context about " +
			"where and when photos were taken.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{
				"collection_id": map[string]any{"type": "string", "description": "The collection ID from list_collections. Use the 'id' field from the collection object returned by list_collections."},
			},
			"required": []string{"collection_id"},
		},
	}, eventsHandler(collections, imageSource))

	mcp.AddTool(s, &mcp.Tool{
		Name: "search_photos",
		Description: "Search a collection's photos using natural language text, visual similarity to another image, " +
			"or similarity to a detected face. This is the primary discovery tool for finding specific photos. Returns " +
			"metadata summaries (file name, date, dimensions, dominant color, location, tags, similarity score) — NOT " +
			"the image data itself. Use get_photo with the returned file_id to retrieve actual images and their embeddable URLs.\n\n" +
			"QUERY TYPES:\n" +
			"- Text search: e.g. 'red car on highway' — uses CLIP embeddings to find semantically similar images, sorted by match quality\n" +
			"- Image similarity: e.g. 'img:1234' — finds photos visually similar to the given image ID\n" +
			"- Face similarity: e.g. 'face:5678' — finds photos containing similar faces\n\n" +
			"COMBINABLE QUALIFIERS (can mix with text search or use standalone):\n" +
			"- tag:name — filter by tag (e.g. 'vacation', 'fav')\n" +
			"- filename:text — filter by filename (supports * and ? wildcards, e.g. 'filename:*.png')\n" +
			"- created:YYYY-MM-DD — filter by date (supports ranges like 'created:2023-01-01..2023-12-31', wildcards like 'created:*-12-25', and operators like 'created:>=2023-06-15')\n" +
			"- t:X — similarity threshold filter (0.15-0.30, where higher = more strict; e.g. 'beach sunset t:0.25')\n" +
			"- dedup:X — filter duplicates by similarity (0-1, e.g. 'dedup:0.9' keeps only photos <90% similar to each other)\n\n" +
			"EXAMPLES:\n" +
			"- 'beach sunset' — semantically search for beach/sunset photos\n" +
			"- 'beach sunset t:0.25' — find beach sunsets with at least 0.25 similarity\n" +
			"- 'created:2023-06..2023-08 tag:vacation' — vacation photos from summer 2023\n" +
			"- 'img:100 tag:fav' — favorited photos similar to image 100\n\n" +
			"PARAMETERS:\n" +
			"- collection_id (required): From list_collections\n" +
			"- query (required): Search query as described above\n" +
			"- sort (optional): Controls result ordering. Default is '-date' (newest first). Options: '-date' " +
			"(newest), '+date' (oldest), '-similarity' (best match first), '-similarity,+date' (best match, then " +
			"newest). The '-' prefix means descending, '+' means ascending. Multiple fields can be combined with commas.\n" +
			"- limit (optional): Maximum number of results. Default is 50. Use a smaller value (10-20) for quick " +
			"previews, or larger (100-200) for comprehensive result sets. Results beyond the limit are silently discarded.\n\n" +
			"WORKFLOW: Call search_photos to find candidates → examine the results → call get_photo on specific file_ids to see actual images and get their embeddable URLs.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{
				"collection_id": map[string]any{"type": "string", "description": "The collection ID from list_collections."},
				"query":         map[string]any{"type": "string", "description": "Search query: natural language text ('red car on highway'), image similarity ('img:1234'), face similarity ('face:5678'), or combined with qualifiers ('beach sunset tag:vacation created:2023-06'). Required."},
				"sort":          map[string]any{"type":       [3]string{"null", "string"}, "description": "Sort order. Default is '-date' (newest first). Options: '-date', '+date', '-similarity', '-similarity,+date'. Descending uses '-', ascending uses '+'."},
				"limit":         map[string]any{"type":       [2]string{"null", "integer"}, "description": "Max results. Default 50. Use 10-20 for quick previews, 100-200 for comprehensive sets."},
			},
			"required": []string{"collection_id", "query"},
		},
	}, searchPhotosHandler(collections, imageSource))

	mcp.AddTool(s, &mcp.Tool{
		Name: "get_photo",
		Description: "Retrieve a photo as a base64-encoded image with rich metadata and embeddable URLs. This is the only tool that returns actual image data.\n\n" +
			"CRITICAL DEFAULT BEHAVIOR — ALWAYS CALL WITH ONLY file_id FIRST:\n" +
			"When you call get_photo with ONLY the file_id parameter (no w, h, crop, or format), it returns a small " +
			"256x256 pixel thumbnail as JPEG. This is the recommended default for: browsing search results, getting a " +
			"quick overview, identifying content at a glance, and most everyday use cases. Small thumbnails are fast, " +
			"efficient, and usually sufficient for identifying what a photo contains.\n\n" +
			"ONLY add extra parameters when you genuinely need more detail:\n" +
			"- w/h: Use ONLY when the thumbnail is too small to make out details. E.g., if you need to read text in a " +
			"sign, identify a distant person, or examine architectural details. Range: 1-4096. Omit both for the default " +
			"256x256 thumbnail.\n" +
			"- format: Rarely needed. Options: 'jpeg' (default, best for photos), 'png' (lossless, good for " +
			"screenshots/diagrams), 'webp' (smaller file size, modern format). Use default jpeg unless you have a specific need.\n" +
			"- crop_x/crop_y/crop_w/crop_h: Use ONLY when you need to zoom into a specific region of the photo. " +
			"Coordinates are in the ORIGINAL image's pixel space (not the output dimensions). All four must be " +
			"specified together. The crop is applied before resizing by w/h. Example: to zoom into a face, you'd " +
			"need to know approximate coordinates from metadata or previous calls.\n\n" +
			"EMBEDDABLE URL (returned in structured metadata — use for markdown, HTML, etc.):\n" +
			"- image_url: Absolute URL to the medium thumbnail (M: 320x320) if available, or original image URL as fallback. Use this for embedding images in markdown or HTML.\n" +
			"- thumbnail[].url: URLs to pre-sized thumbnail variants (S=120px, SM=240px, M=320px, B=640px, XL=1280px)\n" +
			"- faces[].preview_url: Direct URL to each face's cropped preview image (200x200)\n\n" +
			"OUTPUT METADATA (returned alongside the image):\n" +
			"- image_url: Absolute URL to medium thumbnail (M) or original image (for markdown embedding)\n" +
			"- width/height: The rendered output dimensions\n" +
			"- orig_width/orig_height: The original image's native resolution\n" +
			"- path/filename/extension: Original file path details\n" +
			"- video: true if this is a video file\n" +
			"- created_at: Creation date in ISO 8601 format\n" +
			"- tags: Detected semantic tags with file counts\n" +
			"- faces: Detected faces with bounding box coordinates and confidence scores\n" +
			"- latlng: GPS coordinates if available\n" +
			"- location: Reverse-geocoded location string (e.g. 'Paris, France')\n" +
			"- thumbnails: Available thumbnail variants with their sizes and URLs\n\n" +
			"WORKFLOW: Use list_collections → events/search_photos for discovery → get_photo(file_id) for thumbnails → " +
			"get_photo(file_id, w=800, h=600) only when you need to inspect details. Use the returned image_url for markdown embedding.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{
				"file_id": map[string]any{"type": "integer", "description": "The photo file ID (required). Obtain from search_photos results or get_photo output."},
				"w":       map[string]any{"type":       [2]string{"null", "integer"}, "description": "Target width in pixels (1-4096). OMIT for default 256x256 thumbnail. ONLY specify when you need larger output to inspect details that are unclear in the thumbnail."},
				"h":       map[string]any{"type":       [2]string{"null", "integer"}, "description": "Target height in pixels (1-4096). OMIT for default 256x256 thumbnail. ONLY specify when you need larger output to inspect details that are unclear in the thumbnail."},
				"format":  map[string]any{"type":       [2]string{"null", "string"}, "description": "Output format. Default: 'jpeg'. Options: 'jpeg' (recommended for photos, best quality/size balance), 'png' (lossless, use for screenshots/text), 'webp' (smaller files, modern). Rarely need to change from default."},
				"crop_x":  map[string]any{"type":       [2]string{"null", "integer"}, "description": "Crop left edge in ORIGINAL image pixels. Use with crop_y/crop_w/crop_h to zoom into a specific region. Coordinates are in the original image's pixel space, not the output dimensions."},
				"crop_y":  map[string]any{"type":       [2]string{"null", "integer"}, "description": "Crop top edge in ORIGINAL image pixels. Must be used with crop_w and crop_h."},
				"crop_w":  map[string]any{"type":       [2]string{"null", "integer"}, "description": "Crop width in ORIGINAL image pixels. Must be used with crop_x, crop_y, and crop_h."},
				"crop_h":  map[string]any{"type":       [2]string{"null", "integer"}, "description": "Crop height in ORIGINAL image pixels. Must be used with crop_x, crop_y, and crop_w."},
			},
			"required": []string{"file_id"},
		},
	}, getPhotoHandler(collections, imageSource, serverBaseURL))

	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return s
	}, nil)

	// Wrap with panic recovery to prevent server crashes from tool handler panics
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var written bool
		// Wrap ResponseWriter to detect if WriteHeader was called
		wrappedW := &responseWriterWrapper{ResponseWriter: w, wroteHeader: &written}
		defer func() {
			if rec := recover(); rec != nil {
				fmt.Fprintln(os.Stderr, "MCP handler recovered from panic:", rec)
				// Try to write an error response if not already written
				if !written {
					wrappedW.Header().Set("Content-Type", "application/json")
					wrappedW.WriteHeader(http.StatusInternalServerError)
				}
			}
		}()
		h.ServeHTTP(wrappedW, r)
	})

	return &Server{srv: s, handler: wrappedHandler}, nil
}

// --- list_collections ---

type collectionsInput struct{}

type collectionsOutput struct {
	Items []collection.Collection `json:"items"`
}

func listCollections(collections *[]collection.Collection, imageSource *image.Source) mcp.ToolHandlerFor[collectionsInput, collectionsOutput] {
	return func(_ context.Context, _ *mcp.CallToolRequest, _ collectionsInput) (*mcp.CallToolResult, collectionsOutput, error) {
		var items []collection.Collection
		for i := range *collections {
			c := &(*collections)[i]
			c.UpdateIndexedAt(imageSource)
			items = append(items, *c)
		}
		if items == nil {
			items = make([]collection.Collection, 0)
		}
		return nil, collectionsOutput{Items: items}, nil
	}
}

// --- events ---

type eventsInput struct {
	CollectionId string `json:"collection_id" jsonschema:"The collection ID from list_collections. Use the 'id' field from the collection object returned by list_collections."`
}

type eventsOutput struct {
	Events []collection.EventSummary `json:"events"`
}

func eventsHandler(collections *[]collection.Collection, imageSource *image.Source) mcp.ToolHandlerFor[eventsInput, eventsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input eventsInput) (*mcp.CallToolResult, eventsOutput, error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "events handler recovered from panic:", r)
			}
		}()
		// Find the collection
		var coll *collection.Collection
		for i := range *collections {
			if (*collections)[i].Id == input.CollectionId {
				coll = &(*collections)[i]
				break
			}
		}
		if coll == nil {
			return nil, eventsOutput{}, nil
		}

		// Delegate to collection method
		events, err := coll.SplitIntoEvents(ctx, imageSource)
		if err != nil {
			return nil, eventsOutput{}, err
		}
		return nil, eventsOutput{Events: events}, nil
	}
}

// --- search_photos ---

type searchPhotosInput struct {
	CollectionId string  `json:"collection_id" jsonschema:"The collection ID from list_collections."`
	Query        string  `json:"query" jsonschema:"Search query: natural language text ('red car on highway'), image similarity ('img:1234'), face similarity ('face:5678'), or combined with qualifiers ('beach sunset tag:vacation created:2023-06'). Required."`
	Sort         *string `json:"sort" jsonschema:"Sort order. Default: \"-date\" (newest first). Options: \"-date\", \"+date\", \"-similarity\", \"-similarity,+date\". \"-\" = descending, \"+\" = ascending. Multiple fields separated by commas."`
	Limit        *int    `json:"limit" jsonschema:"Max results. Default 50. Use 10-20 for quick previews, 100-200 for comprehensive sets."`
}

type searchPhotosOutput struct {
	Items []collection.SearchResult `json:"items"`
}

func searchPhotosHandler(collections *[]collection.Collection, imageSource *image.Source) mcp.ToolHandlerFor[searchPhotosInput, searchPhotosOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input searchPhotosInput) (*mcp.CallToolResult, searchPhotosOutput, error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintln(os.Stderr, "search_photos handler recovered from panic:", r)
			}
		}()
		// Find the collection
		var coll *collection.Collection
		for i := range *collections {
			if (*collections)[i].Id == input.CollectionId {
				coll = &(*collections)[i]
				break
			}
		}
		if coll == nil {
			return nil, searchPhotosOutput{}, nil
		}

		limit := 50
		if input.Limit != nil && *input.Limit > 0 {
			limit = *input.Limit
		}

		sort := collection.SortType("")
		if input.Sort != nil && *input.Sort != "" {
			sort = collection.SortType(*input.Sort)
		}

		opts := collection.SearchOptions{
			QueryStr: input.Query,
			Sort:     sort,
			Limit:    limit,
		}

		items, _, _, err := coll.Search(ctx, imageSource, opts)
		if err != nil {
			return nil, searchPhotosOutput{}, err
		}
		if items == nil {
			items = make([]collection.SearchResult, 0)
		}
		return nil, searchPhotosOutput{Items: items}, nil
	}
}

// responseWriterWrapper wraps http.ResponseWriter to track if WriteHeader was called.
type responseWriterWrapper struct {
	http.ResponseWriter
	wroteHeader *bool
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	if !*w.wroteHeader {
		*w.wroteHeader = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *responseWriterWrapper) Write(b []byte) (int, error) {
	if !*w.wroteHeader {
		*w.wroteHeader = true
		w.ResponseWriter.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Handler returns the http.Handler for mounting onto a chi router.
func (s *Server) Handler() http.Handler {
	return s.handler
}
