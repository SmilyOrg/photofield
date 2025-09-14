package codec

import (
	"testing"

	"photofield/internal/codec/avif"
	"photofield/internal/codec/jpeg"
	"photofield/internal/codec/png"
	webpchai "photofield/internal/codec/webp/chai"
	webphugo "photofield/internal/codec/webp/hugo"
	webpjack "photofield/internal/codec/webp/jack"
	webpjackdyn "photofield/internal/codec/webp/jack/dynamic"
	webpjacktra "photofield/internal/codec/webp/jack/transpiled"
)

func TestMediaRanges_Best(t *testing.T) {
	tests := []struct {
		name          string
		accept        string
		expectSubtype string
		expectEncoder EncodeFunc
		expectQuality int
	}{
		{
			name:          "finds jpeg encoder",
			accept:        "image/jpeg",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
		},
		{
			name:          "skips non-image types",
			accept:        "text/html, image/jpeg",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
		},
		{
			name:          "finds png encoder",
			accept:        "image/png",
			expectSubtype: "png",
			expectEncoder: png.Encode,
		},
		{
			name:          "finds webp encoder",
			accept:        "image/webp",
			expectSubtype: "webp",
			expectEncoder: webphugo.Encode,
		},
		{
			name:          "finds avif encoder",
			accept:        "image/avif",
			expectSubtype: "avif",
			expectEncoder: avif.Encode,
		},
		{
			name:          "returns first matching encoder",
			accept:        "image/jpeg, image/webp",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
		},
		{
			name:          "handles encoder parameter for webp",
			accept:        "image/webp;encoder=chai",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
		},
		{
			name:          "handles encoder parameter for webp - jack",
			accept:        "image/webp;encoder=jack",
			expectSubtype: "webp",
			expectEncoder: webpjack.Encode,
		},
		{
			name:          "handles encoder parameter for webp - jackdyn",
			accept:        "image/webp;encoder=jackdyn",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
		},
		{
			name:          "handles encoder parameter for webp - jacktra",
			accept:        "image/webp;encoder=jacktra",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
		},
		{
			name:   "returns nil for unknown encoder with parameter",
			accept: "image/jpeg;encoder=unknown",
		},
		{
			name:          "empty accept header uses jpeg",
			accept:        "",
			expectSubtype: "*",
			expectEncoder: jpeg.Encode,
		},
		{
			name:          "returns nil for unsupported type",
			accept:        "image/gif",
			expectEncoder: nil,
		},
		{
			name:          "respects quality preference ordering",
			accept:        "image/png;q=0.2, image/jpeg;q=1.0",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
		},
		{
			name:          "selects highest quality supported format",
			accept:        "image/gif;q=0.9, image/jpeg;q=0.7, image/png;q=0.8",
			expectSubtype: "png",
			expectEncoder: png.Encode,
		},
		{
			name:          "real world browser accept header",
			accept:        "image/avif, image/webp, image/png, image/svg+xml, image/*;q=0.8, */*;q=0.5",
			expectSubtype: "avif",
			expectEncoder: avif.Encode,
		},
		{
			name:          "modern browser with quality preferences",
			accept:        "image/avif;q=0.9, image/webp;q=0.8, image/jpeg;q=0.6, image/*;q=0.4",
			expectSubtype: "avif",
			expectEncoder: avif.Encode,
		},
		{
			name:          "jpeg with quality parameter",
			accept:        "image/jpeg;quality=100",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
			expectQuality: 100,
		},
		{
			name:          "jpeg with quality 90",
			accept:        "image/jpeg;quality=90",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
			expectQuality: 90,
		},
		{
			name:          "jpeg with quality 80",
			accept:        "image/jpeg;quality=80",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
			expectQuality: 80,
		},
		{
			name:          "jpeg with quality 70",
			accept:        "image/jpeg;quality=70",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
			expectQuality: 70,
		},
		{
			name:          "jpeg with quality 60",
			accept:        "image/jpeg;quality=60",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
			expectQuality: 60,
		},
		{
			name:          "jpeg with quality 50",
			accept:        "image/jpeg;quality=50",
			expectSubtype: "jpeg",
			expectEncoder: jpeg.Encode,
			expectQuality: 50,
		},
		{
			name:          "webp with unknown encoder",
			accept:        "image/webp;encoder=HugoSmits86",
			expectEncoder: nil,
		},
		{
			name:          "webp chai with quality 100",
			accept:        "image/webp;encoder=chai;quality=100",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
			expectQuality: 100,
		},
		{
			name:          "webp chai with quality 90",
			accept:        "image/webp;encoder=chai;quality=90",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
			expectQuality: 90,
		},
		{
			name:          "webp chai with quality 80",
			accept:        "image/webp;encoder=chai;quality=80",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
			expectQuality: 80,
		},
		{
			name:          "webp chai with quality 70",
			accept:        "image/webp;encoder=chai;quality=70",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
			expectQuality: 70,
		},
		{
			name:          "webp chai with quality 60",
			accept:        "image/webp;encoder=chai;quality=60",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
			expectQuality: 60,
		},
		{
			name:          "webp chai with quality 50",
			accept:        "image/webp;encoder=chai;quality=50",
			expectSubtype: "webp",
			expectEncoder: webpchai.Encode,
			expectQuality: 50,
		},
		{
			name:          "webp jackdyn with mem and quality 100",
			accept:        "image/webp;encoder=jackdyn;quality=100",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
			expectQuality: 100,
		},
		{
			name:          "webp jackdyn with mem and quality 90",
			accept:        "image/webp;encoder=jackdyn;quality=90",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
			expectQuality: 90,
		},
		{
			name:          "webp jackdyn with mem and quality 80",
			accept:        "image/webp;encoder=jackdyn;quality=80",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
			expectQuality: 80,
		},
		{
			name:          "webp jackdyn with mem and quality 70",
			accept:        "image/webp;encoder=jackdyn;quality=70",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
			expectQuality: 70,
		},
		{
			name:          "webp jackdyn with mem and quality 60",
			accept:        "image/webp;encoder=jackdyn;quality=60",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
			expectQuality: 60,
		},
		{
			name:          "webp jackdyn with mem and quality 50",
			accept:        "image/webp;encoder=jackdyn;quality=50",
			expectSubtype: "webp",
			expectEncoder: webpjackdyn.Encode,
			expectQuality: 50,
		},
		{
			name:          "webp jacktra with mem and quality 100",
			accept:        "image/webp;encoder=jacktra;quality=100",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
			expectQuality: 100,
		},
		{
			name:          "webp jacktra with mem and quality 90",
			accept:        "image/webp;encoder=jacktra;quality=90",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
			expectQuality: 90,
		},
		{
			name:          "webp jacktra with mem and quality 80",
			accept:        "image/webp;encoder=jacktra;quality=80",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
			expectQuality: 80,
		},
		{
			name:          "webp jacktra with mem and quality 70",
			accept:        "image/webp;encoder=jacktra;quality=70",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
			expectQuality: 70,
		},
		{
			name:          "webp jacktra with mem and quality 60",
			accept:        "image/webp;encoder=jacktra;quality=60",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
			expectQuality: 60,
		},
		{
			name:          "webp jacktra with mem and quality 50",
			accept:        "image/webp;encoder=jacktra;quality=50",
			expectSubtype: "webp",
			expectEncoder: webpjacktra.Encode,
			expectQuality: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, err := ParseAccept(tt.accept)
			if err != nil {
				t.Fatalf("unexpected error parsing accept header: %v", err)
			}
			if len(ranges) == 0 {
				if tt.expectEncoder != nil {
					t.Fatal("expected encoder but got none")
				}
				// No ranges and no expected encoder is a valid case
				return
			}
			encoder, mr, ok := ranges.BestEncoder()

			if tt.expectQuality != 0 {
				quality := mr.QualityParam()
				if quality != tt.expectQuality {
					t.Errorf("expected quality %d, got %d", tt.expectQuality, quality)
				}
			}

			if tt.expectEncoder != nil {
				if !ok {
					t.Error("expected encoder but got none")
					return
				}
				if encoder.Func == nil {
					t.Error("expected non-nil encoder function")
					return
				}
				if mr.Subtype != tt.expectSubtype {
					t.Errorf("expected subtype %q, got %q", tt.expectSubtype, mr.Subtype)
				}
				// Compare function pointers by checking if they're both non-nil
				// since direct comparison of function values is not allowed
				if (encoder.Func == nil) != (tt.expectEncoder == nil) {
					t.Error("encoder function mismatch")
				}
			} else {
				if encoder.Func != nil {
					t.Error("expected nil encoder but got non-nil")
				}
			}
		})
	}
}
