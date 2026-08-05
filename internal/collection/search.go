package collection

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/golang/geo/s2"

	"photofield/internal/ai"
	"photofield/internal/image"
	"photofield/internal/layout"
	"photofield/internal/search"
)

// SortType represents a sort specification for search results.
// Supports +/- prefixes for descending/ascending, and supports
// multiple sort fields (e.g. "-similarity,+date").
type SortType string

// SortOrder maps a SortType to an image.ListOrder.
// Returns the first recognized order, or DateDesc as default.
func SortOrder(s SortType) image.ListOrder {
	if s == "" {
		return image.DateDesc
	}
	lo := layout.OrderFromSort(string(s))
	return image.ListOrder(lo)
}

// SortOrders parses a sort string and returns a slice of ListOrders.
// The primary sort is the first element; secondary sorts are appended.
// Supports formats like "-similarity,+date" or just "-date".
func SortOrders(s SortType) []image.ListOrder {
	if s == "" {
		return []image.ListOrder{image.DateDesc}
	}
	orders := make([]image.ListOrder, 0)
	parts := strings.Split(string(s), ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo := layout.OrderFromSort(part)
		if lo != layout.None {
			orders = append(orders, image.ListOrder(lo))
		}
	}
	if len(orders) == 0 {
		return []image.ListOrder{image.DateDesc}
	}
	return orders
}

// SortPrimaryOrder returns the primary sort order.
func SortPrimaryOrder(s SortType) image.ListOrder {
	orders := SortOrders(s)
	if len(orders) == 0 {
		return image.DateDesc
	}
	return orders[0]
}

// SortIsSimilarity returns true if the primary sort is similarity-based.
func SortIsSimilarity(s SortType) bool {
	lo := SortPrimaryOrder(s)
	return lo == image.SimilarityDesc || lo == image.SimilarityAsc
}

// SearchOptions holds the parameters for a collection search.
type SearchOptions struct {
	QueryStr string
	Sort     SortType
	Limit    int
}

// SearchResult represents a single search result with metadata.
type SearchResult struct {
	Id         int32    `json:"id"`
	FileName   string   `json:"file_name"`
	DateTime   string   `json:"datetime,omitempty"`
	Width      int      `json:"width,omitempty"`
	Height     int      `json:"height,omitempty"`
	Color      string   `json:"color,omitempty"`  // hex color
	Location   string   `json:"location,omitempty"` // reverse-geocoded, if geo available
	Similarity float32  `json:"similarity,omitempty"`
	Tags       []string `json:"tags,omitempty"`     // tags on this photo
}

// Search searches a collection's photos by text, image reference, face reference,
// or structured qualifiers. Returns metadata and similarity scores.
func (collection *Collection) Search(
	ctx context.Context,
	source *image.Source,
	opts SearchOptions,
) ([]SearchResult, []search.Token, []search.FieldMeta, error) {
	// 1. Parse query
	var tokens []search.Token
	var expr search.Expression
	var parseErr error

	if opts.QueryStr != "" {
		q, err := search.Parse(opts.QueryStr)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse failed: %w", err)
		}
		tokens = q.Tokens()
		expr, parseErr = q.Expression()
	}

	// 2. Resolve embeddings
	var imageEmbedding ai.Embedding
	var faceEmbedding ai.Embedding

	if parseErr == nil && expr.Image.Present {
		emb, err := source.GetImageEmbedding(image.ImageId(expr.Image.Value))
		if err != nil {
			return nil, tokens, expr.Errors, fmt.Errorf("image embed failed: %w", err)
		}
		imageEmbedding = emb
	}

	if parseErr == nil && imageEmbedding == nil && expr.Face.Present {
		emb, err := source.GetFaceEmbedding(int(expr.Face.Value))
		if err != nil {
			return nil, tokens, expr.Errors, fmt.Errorf("face embed failed: %w", err)
		}
		faceEmbedding = emb
	}

	if parseErr == nil && imageEmbedding == nil && expr.Text != "" {
		emb, err := source.Clip.EmbedText(expr.Text)
		if err != nil {
			return nil, tokens, expr.Errors, fmt.Errorf("text embed failed: %w", err)
		}
		imageEmbedding = emb
	}

	// 3. Determine sort order
	order := SortPrimaryOrder(opts.Sort)

	// No default threshold — let all results through and let the caller
	// control filtering via `t:X` if desired. (The original loadScene used
	// 0.262 for non-similarity sorts but that was too aggressive for some
	// datasets.)

	// 4. Query DB
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	infos, _ := collection.GetInfos(source, image.ListOptions{
		OrderBy:        order,
		Limit:          limit,
		Expression:     expr,
		ImageEmbedding: imageEmbedding,
		FaceEmbedding:  faceEmbedding,
	})

	// 5. Collect results
	results := make([]SearchResult, 0)
	var lastLocTime time.Time
	var lastLatLng s2.LatLng

	for info := range infos {
		res := SearchResult{
			Id:         int32(info.Id),
			FileName:   filepath.Base(getImagePath(source, info.Id)),
			Similarity: info.Similarity,
		}

		if !info.DateTime.IsZero() {
			res.DateTime = info.DateTime.Format(time.RFC3339)
		}
		res.Width = info.Width
		res.Height = info.Height

		// Color as hex string
		if info.Color != 0 {
			res.Color = fmt.Sprintf("#%06x", info.Color&0xFFFFFF)
		}

		// Reverse-geocode (same logic as events: 15 min / 1 km gap)
		if source.Geo != nil && source.Geo.Available() && image.IsValidLatLng(info.LatLng) {
			lastLocCheck := lastLocTime.Sub(info.DateTime)
			if lastLocCheck < 0 {
				lastLocCheck = -lastLocCheck
			}
			queryLocation := lastLocTime.IsZero() || lastLocCheck > 15*time.Minute
			if queryLocation {
				lastLocTime = info.DateTime
				dist := image.AngleToKm(lastLatLng.Distance(info.LatLng))
				if dist > 1.0 {
					location, err := source.Geo.ReverseGeocode(ctx, info.LatLng)
					if err == nil {
						lastLatLng = info.LatLng
						res.Location = location // assign to current photo
					}
				}
			}
		}

		// Tags
		tagNames := make([]string, 0)
		for t := range source.ListImageTags(info.Id) {
			tagNames = append(tagNames, t.Name)
		}
		sort.Strings(tagNames)
		res.Tags = tagNames

		results = append(results, res)
	}

	if results == nil {
		results = make([]SearchResult, 0)
	}

	return results, tokens, expr.Errors, parseErr
}

func getImagePath(source *image.Source, id image.ImageId) string {
	path, _ := source.GetImagePath(id)
	return path
}
