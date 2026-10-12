package main

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"photofield/internal/image"
	"photofield/internal/search"
)

var jst = time.FixedZone("JST", 9*3600)

// writePhotos populates a fresh cache database with photos that have
// different stored timezone offsets, as would result from EXIF/XMP
// decoding:
//
//	"/photos/camera-no-offset.jpg" — offsetless EXIF date stored as UTC
//	zone 0 (the normal case for camera DateTimeOriginal)
//	"/photos/jst-early.jpg"        — XMP date with +09:00 offset:
//	2025-06-01 03:30 JST == 2025-05-31 18:30 UTC
//	"/photos/jst-late.jpg"         — XMP date with +09:00 offset:
//	2025-06-01 23:30 JST == 2025-06-01 14:30 UTC
func writePhotos(t *testing.T) *image.Database {
	t.Helper()
	db := image.NewDatabase(filepath.Join(t.TempDir(), "photofield.db"), migrations)

	photos := []struct {
		path string
		date time.Time
	}{
		{"/photos/camera-no-offset.jpg", time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)},
		{"/photos/jst-early.jpg", time.Date(2025, 6, 1, 3, 30, 0, 0, jst)},
		{"/photos/jst-late.jpg", time.Date(2025, 6, 1, 23, 30, 0, 0, jst)},
	}
	for _, p := range photos {
		if err := db.Write(p.path, image.Info{}, image.AppendPath); err != nil {
			t.Fatalf("write %s: %v", p.path, err)
		}
		if err := db.Write(p.path, image.Info{DateTime: p.date}, image.UpdateMeta); err != nil {
			t.Fatalf("write meta %s: %v", p.path, err)
		}
	}
	<-db.CommitBarrier()
	return db
}

func listPaths(t *testing.T, db *image.Database, query string) []string {
	t.Helper()

	parsed, err := search.Parse(query)
	if err != nil {
		t.Fatalf("parse %q: %v", query, err)
	}
	expr, err := parsed.Expression()
	if err != nil {
		t.Fatalf("expression %q: %v", query, err)
	}

	var paths []string
	results, _ := db.List([]string{"/photos"}, image.ListOptions{
		Expression: expr,
	})
	for result := range results {
		path, ok := db.GetPathFromId(result.Id)
		if !ok {
			t.Fatalf("missing path for image id %d", result.Id)
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

func TestListCreatedQualifierMatchesLocalDate(t *testing.T) {
	testCases := []struct {
		name   string
		query  string
		expect []string
	}{
		{
			name:   "offsetless date matches camera wall date",
			query:  "created:2025-06-01",
			expect: []string{"/photos/camera-no-offset.jpg", "/photos/jst-early.jpg", "/photos/jst-late.jpg"},
		},
		{
			name:   "early JST photo (UTC date is previous day) matches local date",
			query:  "created:2025-05-31",
			expect: []string{},
		},
		{
			name:   "range end excludes next local day",
			query:  "created:2025-05-31..2025-06-02",
			expect: []string{"/photos/camera-no-offset.jpg", "/photos/jst-early.jpg", "/photos/jst-late.jpg"},
		},
		{
			name:   "wildcard month-day matches local date",
			query:  "created:*-06-01",
			expect: []string{"/photos/camera-no-offset.jpg", "/photos/jst-early.jpg", "/photos/jst-late.jpg"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := writePhotos(t)
			defer db.Close()
			paths := listPaths(t, db, tc.query)
			if !slices.Equal(paths, tc.expect) {
				t.Fatalf("query %q paths = %v, want %v", tc.query, paths, tc.expect)
			}
		})
	}
}
