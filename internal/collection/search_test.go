package collection

import (
	"testing"

	"photofield/internal/image"
)

func TestPrimaryOrderFor(t *testing.T) {
	cases := []struct {
		name         string
		sort         SortType
		hasEmbedding bool
		want         image.ListOrder
	}{
		{"empty sort, no embedding defaults to newest first", "", false, image.DateDesc},
		{"empty sort, embedding defaults to best matches first", "", true, image.SimilarityDesc},
		{"explicit sort wins over embedding default", "+date", true, image.DateAsc},
		{"explicit similarity sort preserved", "-similarity", false, image.SimilarityDesc},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := primaryOrderFor(tc.sort, tc.hasEmbedding); got != tc.want {
				t.Fatalf("primaryOrderFor(%q, %v) = %v, want %v", tc.sort, tc.hasEmbedding, got, tc.want)
			}
		})
	}
}
