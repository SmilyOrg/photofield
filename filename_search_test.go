package main

import (
	"path/filepath"
	"slices"
	"testing"

	"photofield/internal/image"
	"photofield/internal/search"
)

func collectListedPaths(t *testing.T, db *image.Database, expr search.Expression) []string {
	t.Helper()

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

func TestListFilenameQualifier(t *testing.T) {
	db := image.NewDatabase(filepath.Join(t.TempDir(), "photofield.db"), migrations)
	defer db.Close()

	for _, path := range []string{
		"/photos/mexico-trip-01.jpg",
		"/photos/new-york-subway-map.png",
		"/photos/photo_01.jpg",
	} {
		if err := db.Write(path, image.Info{}, image.AppendPath); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	<-db.CommitBarrier()

	testCases := []struct {
		name   string
		query  string
		expect []string
	}{
		{
			name:   "contains by default",
			query:  "filename:mexico",
			expect: []string{"/photos/mexico-trip-01.jpg"},
		},
		{
			name:   "supports star wildcards",
			query:  "filename:*.png",
			expect: []string{"/photos/new-york-subway-map.png"},
		},
		{
			name:   "supports question wildcards",
			query:  "filename:photo?01*",
			expect: []string{"/photos/photo_01.jpg"},
		},
		{
			name:   "escapes like metacharacters",
			query:  "filename:photo_01",
			expect: []string{"/photos/photo_01.jpg"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := search.Parse(tc.query)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.query, err)
			}
			expr, err := query.Expression()
			if err != nil {
				t.Fatalf("expression %q: %v", tc.query, err)
			}
			paths := collectListedPaths(t, db, expr)
			if !slices.Equal(paths, tc.expect) {
				t.Fatalf("query %q paths = %v, want %v", tc.query, paths, tc.expect)
			}
		})
	}
}
