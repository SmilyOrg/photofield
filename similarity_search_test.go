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

// unitEmbAtSim builds a stored clip embedding whose cosine similarity with the
// unit query vector (1,0) is `sim`: the unit vector (sim, sqrt(1-sim²)).
func unitEmbAtSim(t *testing.T, sim float32) (x, y float32) {
	t.Helper()
	return sim, float32(math.Sqrt(float64(1 - sim*sim)))
}

// embeddingBytes encodes a 2D vector normalized to unit length as the
// little-endian float16 byte blob the clip_emb table stores. The inverse
// norm of a unit vector is 1.0, whose float16 bits are 0x3C00.
// (Shared helper — also used by the dedup search tests, #202.)
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

// writePhotoEmbs writes 5 photos dated one day apart (01 oldest … 05 newest)
// and attaches clip embeddings with the given similarities to the query
// vector. Returns the db and the id for each path.
func writePhotoEmbs(t *testing.T, sims []float32) (*image.Database, map[string]image.ImageId) {
	t.Helper()
	db := image.NewDatabase(filepath.Join(t.TempDir(), "photofield.db"), migrations)
	t.Cleanup(db.Close)

	paths := []string{
		"/photos/01-old.jpg", "/photos/02.jpg", "/photos/03.jpg", "/photos/04.jpg", "/photos/05-new.jpg",
	}
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	for i, p := range paths {
		if err := db.Write(p, image.Info{}, image.AppendPath); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		if err := db.Write(p, image.Info{DateTime: base.AddDate(0, 0, i)}, image.UpdateMeta); err != nil {
			t.Fatalf("write meta %s: %v", p, err)
		}
	}
	<-db.CommitBarrier()

	ids := map[string]image.ImageId{}
	for pair := range db.ListIdPaths([]string{"/photos"}, 0) {
		ids[pair.Path] = pair.Id
	}
	for i, p := range paths {
		id, ok := ids[p]
		if !ok {
			t.Fatalf("no id for %s", p)
		}
		x, y := unitEmbAtSim(t, sims[i])
		if err := db.WriteAI(id, embeddingBytes(t, x, y)); err != nil {
			t.Fatalf("write ai for %s: %v", p, err)
		}
	}
	<-db.CommitBarrier()
	return db, ids
}

func pathOf(t *testing.T, db *image.Database, id image.ImageId) string {
	t.Helper()
	p, ok := db.GetPathFromId(id)
	if !ok {
		t.Fatalf("missing path for id %d", id)
	}
	return p
}

// The two newest photos are below the threshold while three older ones pass.
// The SQL LIMIT used to pre-clip the stream to the two newest rows, so the
// threshold filter saw only the two failing rows and the query returned 0.
// The limit must be applied in Go AFTER thresholding, across the full set.
func TestListThresholdLimitSpansFullSet(t *testing.T) {
	db, _ := writePhotoEmbs(t, []float32{0.5, 0.8, 0.9, 0.2, 0.1})

	parsed, err := search.Parse("t:0.3")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	expr, err := parsed.Expression()
	if err != nil {
		t.Fatalf("expression: %v", err)
	}
	x, y := unitEmbAtSim(t, 1.0)

	var got []string
	results, _ := db.List([]string{"/photos"}, image.ListOptions{
		Expression:     expr,
		ImageEmbedding: embeddingBytes(t, x, y),
		OrderBy:        image.DateDesc,
		Limit:          2,
	})
	for result := range results {
		got = append(got, pathOf(t, db, result.Id))
	}

	// 03 (newest passing) then 02; 01 also passes but is cut by the limit.
	want := []string{"/photos/03.jpg", "/photos/02.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("t:0.3 -date limit 2 = %v, want %v (threshold must apply across the full set, limit afterwards)", got, want)
	}
}

// Similarity orders already scored the full set in Go; pin that down,
// including that the limit keeps the top N by similarity, not by date.
func TestListSimilarityOrderLimitTopMatches(t *testing.T) {
	db, ids := writePhotoEmbs(t, []float32{0.5, 0.8, 0.9, 0.2, 0.1})
	_ = ids

	x, y := unitEmbAtSim(t, 1.0)

	var got []string
	results, _ := db.List([]string{"/photos"}, image.ListOptions{
		ImageEmbedding: embeddingBytes(t, x, y),
		OrderBy:        image.SimilarityDesc,
		Limit:          3,
	})
	for result := range results {
		got = append(got, pathOf(t, db, result.Id))
	}

	want := []string{"/photos/03.jpg", "/photos/02.jpg", "/photos/01-old.jpg"}
	if !slices.Equal(got, want) {
		t.Fatalf("-similarity limit 3 = %v, want %v", got, want)
	}
}
