package main

import (
	"encoding/binary"
	"math"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/x448/float16"
	"photofield/internal/ai"
	"photofield/internal/image"
	"photofield/internal/search"
)

// embeddingBytes encodes a 2D vector normalized to unit length as the
// little-endian float16 byte blob the clip_emb table stores. The inverse
// norm of a unit vector is 1.0, whose float16 bits are 0x3C00.
func embeddingBytes(t *testing.T, x, y float32) ai.Embedding {
	t.Helper()
	n := float32(math.Sqrt(float64(x*x + y*y)))
	f := [2]uint16{
		float16.Fromfloat32(x / n).Bits(),
		float16.Fromfloat32(y / n).Bits(),
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint16(buf[0:2], f[0])
	binary.LittleEndian.PutUint16(buf[2:4], f[1])
	return ai.FromRaw(buf, 0x3C00)
}

func TestListDeduplicateSuppressesNonAdjacentNearDuplicates(t *testing.T) {
	db := image.NewDatabase(filepath.Join(t.TempDir(), "photofield.db"), migrations)
	defer db.Close()

	// Three photos dated one day apart so DateAsc streams them A, B, A':
	// A and A' are near-duplicates (cos ≈ 0.912) separated by an unrelated
	// photo B.
	paths := []string{"/photos/01-a.jpg", "/photos/02-b.jpg", "/photos/03-a-dup.jpg"}
	dates := []time.Time{
		time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 2, 12, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 3, 12, 0, 0, 0, time.UTC),
	}
	embs := [3][2]float32{{1, 0}, {0, 1}, {9, 4}}

	for i, p := range paths {
		if err := db.Write(p, image.Info{}, image.AppendPath); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		if err := db.Write(p, image.Info{DateTime: dates[i]}, image.UpdateMeta); err != nil {
			t.Fatalf("write meta %s: %v", p, err)
		}
	}
	<-db.CommitBarrier()

	// Resolve ids by path and attach embeddings.
	ids := map[string]image.ImageId{}
	for pair := range db.ListIdPaths([]string{"/photos"}, 0) {
		ids[pair.Path] = pair.Id
	}
	for i, p := range paths {
		id, ok := ids[p]
		if !ok {
			t.Fatalf("no id for %s", p)
		}
		if err := db.WriteAI(id, embeddingBytes(t, embs[i][0], embs[i][1])); err != nil {
			t.Fatalf("write ai for %s: %v", p, err)
		}
	}
	<-db.CommitBarrier()

	parsed, err := search.Parse("dedup:0.9")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	expr, err := parsed.Expression()
	if err != nil {
		t.Fatalf("expression: %v", err)
	}

	var got []string
	results, _ := db.List([]string{"/photos"}, image.ListOptions{
		Expression: expr,
		OrderBy:    image.DateAsc,
	})
	for result := range results {
		path, ok := db.GetPathFromId(result.Id)
		if !ok {
			t.Fatalf("missing path for id %d", result.Id)
		}
		got = append(got, path)
	}
	slices.Sort(got)

	want := []string{"/photos/01-a.jpg", "/photos/02-b.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("dedup:0.9 paths = %v, want %v (near-duplicate 03-a-dup.jpg must be suppressed even though 02-b.jpg separates it from 01-a.jpg)", got, want)
	}
}
