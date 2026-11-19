package main

import (
	"slices"
	"testing"

	"photofield/internal/image"
	"photofield/internal/search"
)

// Photos 01..05 are dated one day apart (05 is the newest). Their embeddings
// pair up as 04 = 05 and 02 = 03, with 01 unlike all the others, so dedup:0.95
// leaves one photo of each pair.
var dedupSims = []float32{0.0, 0.5, 0.5, 0.9, 0.9}

func listDedup(t *testing.T, db *image.Database, query string, orderBy image.ListOrder, limit int) []string {
	t.Helper()
	parsed, err := search.Parse(query)
	if err != nil {
		t.Fatalf("parse %q: %v", query, err)
	}
	expr, err := parsed.Expression()
	if err != nil {
		t.Fatalf("expression %q: %v", query, err)
	}

	var got []string
	results, _ := db.List([]string{"/photos"}, image.ListOptions{
		Expression: expr,
		OrderBy:    orderBy,
		Limit:      limit,
	})
	for result := range results {
		got = append(got, pathOf(t, db, result.Id))
	}
	return got
}

// A standalone dedup: has no query embedding, but it still filters in Go. The
// SQL LIMIT used to cut the stream to the first rows before dedup ran, so the
// result came back short even though more unique photos existed further on.
func TestListDedupLimitSpansFullSet(t *testing.T) {
	db, _ := writePhotoEmbs(t, dedupSims)

	tests := []struct {
		name    string
		orderBy image.ListOrder
		limit   int
		want    []string
	}{
		{"newest first, limit 2", image.DateDesc, 2, []string{"/photos/05-new.jpg", "/photos/03.jpg"}},
		{"newest first, limit 3", image.DateDesc, 3, []string{"/photos/05-new.jpg", "/photos/03.jpg", "/photos/01-old.jpg"}},
		{"oldest first, limit 3", image.DateAsc, 3, []string{"/photos/01-old.jpg", "/photos/02.jpg", "/photos/04.jpg"}},
		{"limit above the unique count", image.DateDesc, 4, []string{"/photos/05-new.jpg", "/photos/03.jpg", "/photos/01-old.jpg"}},
		{"no limit", image.DateDesc, 0, []string{"/photos/05-new.jpg", "/photos/03.jpg", "/photos/01-old.jpg"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listDedup(t, db, "dedup:0.95", tt.orderBy, tt.limit)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("dedup:0.95 limit %d = %v, want %v", tt.limit, got, tt.want)
			}
		})
	}
}

// Without dedup the limit still applies to the rows in order, nothing is
// dropped for being similar.
func TestListLimitWithoutDedupKeepsSimilarPhotos(t *testing.T) {
	db, _ := writePhotoEmbs(t, dedupSims)

	got := listDedup(t, db, "", image.DateDesc, 2)
	want := []string{"/photos/05-new.jpg", "/photos/04.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("limit 2 = %v, want %v", got, want)
	}
}

// A limit of 1 stops reading after the first kept row, which has to be
// emitted exactly once and not lost or repeated.
func TestListDedupLimitOne(t *testing.T) {
	db, _ := writePhotoEmbs(t, dedupSims)

	for _, orderBy := range []image.ListOrder{image.DateDesc, image.DateAsc} {
		got := listDedup(t, db, "dedup:0.95", orderBy, 1)
		if len(got) != 1 {
			t.Fatalf("order %v limit 1 = %v, want exactly one photo", orderBy, got)
		}
	}
}

// A threshold at 1.0 only drops exact copies, so unlike photos all stay and
// the limit applies as usual.
func TestListDedupKeepsUnlikePhotos(t *testing.T) {
	db, _ := writePhotoEmbs(t, []float32{0.0, 0.2, 0.4, 0.6, 0.8})

	got := listDedup(t, db, "dedup:0.99", image.DateDesc, 4)
	want := []string{"/photos/05-new.jpg", "/photos/04.jpg", "/photos/03.jpg", "/photos/02.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("dedup:0.99 limit 4 = %v, want %v", got, want)
	}
}
