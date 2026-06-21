// Package mcp provides a Model Context Protocol (MCP) server for the
// photofield application, mounted onto the chi HTTP router.
package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"photofield/internal/collection"
	"photofield/internal/image"
)

// Server holds the MCP server instance and its chi-mountable HTTP handler.
type Server struct {
	srv       *mcp.Server
	handler   http.Handler
	baseURL   atomic.Value // set from request Host header per request (stores string)
	apiPrefix string       // e.g. "/api" — used for constructing file URLs
}

// New creates a new MCP server for photofield with the given data sources
// and registers all available tools. The base URL is derived at request time
// from the incoming request's Host header, with `addr` used as a fallback
// default (derived from the listener address). `apiPrefix` is the HTTP route
// prefix for file endpoints (e.g. "/api"). Callers should mount handler()
// on a chi router, e.g.:
//
//	r.Mount("/mcp", s.handler())
func New(collections *[]collection.Collection, imageSource *image.Source, addr, apiPrefix string) (*Server, error) {
	sdkSrv := mcp.NewServer(&mcp.Implementation{
		Name:    "photofield",
		Version: "dev",
	}, nil)

	// Handler closures capture collections, imageSource, and a pointer to this Server
	// so they can read the current base URL at request time.
	srv := &Server{srv: sdkSrv, apiPrefix: apiPrefix}

	mcp.AddTool(sdkSrv, &mcp.Tool{
		Name: "list_collections",
		Description: "List all photo collections. Use this first — the collection_id from the response is required by all other tools. " +
			"If indexed_count is 0 or indexed_at is missing, the collection has not been indexed yet.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, listCollections(collections, imageSource))

	mcp.AddTool(sdkSrv, &mcp.Tool{
		Name: "events",
		Description: "Split a collection's photos into chronological events. Returns metadata summaries (photo count, date ranges, location count) — NOT the photo images themselves. " +
			"Use after list_collections, before search_photos, to get high-level context about where and when photos were taken.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"collection_id": map[string]any{"type": "string", "description": "The collection ID from list_collections. Use the 'id' field from the collection object returned by list_collections."},
			},
			"required": []string{"collection_id"},
		},
	}, eventsHandler(collections, imageSource))

	mcp.AddTool(sdkSrv, &mcp.Tool{
		Name: "search_photos",
		Description: "Search a collection's photos by text, image reference (img:ID), or face reference (face:ID). Returns metadata summaries — NOT the image data. " +
			"⚠️ Use get_photo(file_id) to verify key results visually before showing photos to the user — metadata and similarity scores can be misleading. " +
			"Use your judgment on when to verify: for single specific results, definitely check; for browsing large sets, verify only the top matches.\n\n" +
			"Results beyond the limit are silently discarded. Use get_photo_metadata on results to get preview_url for markdown embedding, or get_photo for the full image.\n\n" +
			"QUERY TYPES:\n" +
			"- Text search: e.g. 'red car on highway' — uses CLIP embeddings, sorted by match quality\n" +
			"- Image similarity: e.g. 'img:1234' — finds photos visually similar to the given image ID\n" +
			"- Face similarity: e.g. 'face:5678' — finds photos containing similar faces\n\n" +
			"COMBINABLE QUALIFIERS (mix with text or use standalone):\n" +
			"- tag:name — filter by tag (e.g. 'vacation', 'fav')\n" +
			"- filename:text — filter by filename (supports * and ? wildcards, e.g. 'filename:*.png')\n" +
			"- created:YYYY-MM-DD — filter by date (supports ranges like 'created:2023-01-01..2023-12-31', wildcards like 'created:*-12-25', and operators like 'created:>=2023-06-15')\n" +
			"- t:X — similarity threshold filter (0.15-0.30, where higher = more strict; e.g. 'beach sunset t:0.25')\n" +
			"- dedup:X — filter duplicates by similarity (0-1, e.g. 'dedup:0.9' keeps only photos <90% similar to each other)\n\n" +
			"SORT OPTIONS (passed as the 'sort' parameter):\n" +
			"- -date (default) — newest first\n" +
			"- +date — oldest first\n" +
			"- -similarity — best matches first\n" +
			"- +similarity — worst matches first\n" +
			"- +shuffle-hourly — random within each hour\n" +
			"- +shuffle-daily — random within each day\n" +
			"- +shuffle-weekly — random within each week\n" +
			"- +shuffle-monthly — random within each month\n" +
			"- Multiple fields: e.g. '-similarity,+date' (sort by similarity, break ties newest first)\n\n" +
			"EXAMPLES:\n" +
			"- 'beach sunset' — semantically search for beach/sunset photos\n" +
			"- 'beach sunset t:0.25' — find beach sunsets with at least 0.25 similarity\n" +
			"- 'created:2023-06..2023-08 tag:vacation' — vacation photos from summer 2023\n" +
			"- 'img:100 tag:fav' — favorited photos similar to image 100\n" +
			"- 'dog' sort:-similarity — dogs sorted by relevance\n" +
			"- 'dog' sort:+shuffle-daily — random order, but grouped by day\n" +
			"- 'portrait' sort:-similarity,+date — best portraits first, newest tiebreak\n" +
			"- 'filename:IMG_*.jpg' — all IMG_ photos, oldest first\n\n" +
			"VERIFICATION: For single specific results, call get_photo on top matches to confirm content. For large browse results, verify the top 1-3 matches before presenting.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"collection_id": map[string]any{"type": "string", "description": "The collection ID from list_collections."},
				"query":         map[string]any{"type": "string", "description": "Search query: natural language text, image similarity (img:ID), or face similarity (face:ID)."},
				"sort":          map[string]any{"type": [2]string{"null", "string"}, "description": "Sort order. '-date' (newest) by default. Options: +date, -similarity, +similarity, +shuffle-hourly, +shuffle-daily, +shuffle-weekly, +shuffle-monthly, or comma-joined like '-similarity,+date'."},
				"limit":         map[string]any{"type": [2]string{"null", "integer"}, "description": "Max results. Default 50. Results beyond limit are silently discarded."},
			},
			"required": []string{"collection_id", "query"},
		},
	}, searchPhotosHandler(collections, imageSource))

	mcp.AddTool(sdkSrv, &mcp.Tool{
		Name: "get_photo_metadata",
		Description: "Retrieve structured photo metadata (dimensions, path, dates, tags, faces, location, URLs). " +
			"⚠️ Metadata alone may not be reliable — if you're about to show a photo to the user based on metadata or search results, call get_photo(file_id) to visually confirm it actually contains what you claim. " +
			"Tags, location, and similarity scores can be wrong or misleading. " +
			"Returns preview_url and original_url fields. " +
			"Do NOT output raw HTML (<a><img>) or bare URLs to display photos — the MCP client will not render them. " +
			"Use preview_url directly in markdown syntax (![alt](url)). " +
			"Use after list_collections, events, or search_photos to inspect details on specific file_ids.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_id": map[string]any{"type": "integer", "description": "The photo file ID (required). Obtain from search_photos results."},
			},
			"required": []string{"file_id"},
		},
	}, getPhotoMetadataHandler(imageSource, srv))

	mcp.AddTool(sdkSrv, &mcp.Tool{
		Name: "get_photo",
		Description: "Retrieve a photo as a base64-encoded image. This is the only tool that returns actual image data. " +
			"Use this as a verification tool — call get_photo(file_id) on search results to visually confirm the photo contains what you expect before showing it to the user. " +
			"Metadata and search scores can be misleading, so visual confirmation is recommended for key results.\n\n" +
			"⚠️ DO NOT output raw HTML (<img src=...>) or bare image URLs — the MCP client will not render them. Always call get_photo(file_id) instead. " +
			"Default (file_id only): 256x256 JPEG thumbnail — fast and usually sufficient for verification. " +
			"Format: jpeg (default), png, webp. " +
			"Crop params (crop_x/y/w/h) are in original image pixel coordinates; all four must be specified together. " +
			"Only pass optional parameters (w, h, crop) when the thumbnail is too small to verify details.\n\n" +
			"KEY RULE: Always call get_photo on any photo you're about to show the user or confirm in your response. " +
			"Use your judgment on when to verify for intermediate/browsing results — check top matches, but don't feel you must check every single result.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_id": map[string]any{"type": "integer", "description": "The photo file ID."},
				"w":       map[string]any{"type": [2]string{"null", "integer"}, "description": "Target width in pixels (1-4096). Omit unless investigating details — always start with the default 256x256 thumbnail."},
				"h":       map[string]any{"type": [2]string{"null", "integer"}, "description": "Target height in pixels (1-4096). Omit unless investigating details — always start with the default 256x256 thumbnail."},
				"format":  map[string]any{"type": [2]string{"null", "string"}, "description": "Output format. Default: jpeg. Options: jpeg, png, webp."},
				"crop_x":  map[string]any{"type": [2]string{"null", "integer"}, "description": "Crop left edge in original image pixels. Must specify all four crop params together."},
				"crop_y":  map[string]any{"type": [2]string{"null", "integer"}, "description": "Crop top edge in original image pixels."},
				"crop_w":  map[string]any{"type": [2]string{"null", "integer"}, "description": "Crop width in original image pixels."},
				"crop_h":  map[string]any{"type": [2]string{"null", "integer"}, "description": "Crop height in original image pixels."},
			},
			"required": []string{"file_id"},
		},
	}, getPhotoHandler(imageSource, srv))

	h := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return sdkSrv
	}, nil)

	// Derive a default base URL from the listener address for fallback when
	// the Host header is absent (e.g. behind certain reverse proxies).
	var fallbackAddr string
	if addr != "" {
		_, p, err := net.SplitHostPort(addr)
		if err != nil {
			p = addr // might be a bare port like "8080"
		}
		if p == "" {
			p = "8080"
		}
		fallbackAddr = net.JoinHostPort("localhost", p)
	} else {
		fallbackAddr = "localhost:8080"
	}

	// Wrap with panic recovery to prevent server crashes from tool handler panics.
	// Also extract the Host header from each request and store it in srv.baseURL
	// so that tool handlers can construct absolute image URLs.
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if host == "" {
			host = fallbackAddr
		}
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		srv.baseURL.Store(scheme + "://" + host)

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

	// Initialize baseURL with the fallback default; the wrappedHandler
	// overwrites it per-request.
	srv.baseURL.Store("http://" + fallbackAddr)

	srv.handler = wrappedHandler
	return srv, nil
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
	CollectionId string `json:"collection_id" jsonschema:"The collection ID from list_collections."`
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
			return nil, eventsOutput{}, fmt.Errorf("collection not found: %s", input.CollectionId)
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
	Query        string  `json:"query" jsonschema:"Search query: natural language text, image similarity (img:ID), or face similarity (face:ID)."`
	Sort         *string `json:"sort" jsonschema:"Sort order. '-date' (newest) by default. Options: +date, -similarity, +similarity, +shuffle-hourly, +shuffle-daily, +shuffle-weekly, +shuffle-monthly, or comma-joined like '-similarity,+date'."`
	Limit        *int    `json:"limit" jsonschema:"Max results. Default 50. Results beyond limit are silently discarded."`
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
			return nil, searchPhotosOutput{}, fmt.Errorf("collection not found: %s", input.CollectionId)
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
