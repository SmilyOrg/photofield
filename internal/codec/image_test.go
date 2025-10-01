package codec

import (
	"testing"
)

func TestMediaRanges_Best(t *testing.T) {
	tests := []struct {
		name              string
		accept            string
		expectEncoderType *EncoderType
		expectQuality     int
	}{
		{
			name:              "finds jpeg encoder",
			accept:            "image/jpeg",
			expectEncoderType: &EncoderType{"jpeg", ""},
		},
		{
			name:              "skips non-image types",
			accept:            "text/html, image/jpeg",
			expectEncoderType: &EncoderType{"jpeg", ""},
		},
		{
			name:              "finds png encoder",
			accept:            "image/png",
			expectEncoderType: &EncoderType{"png", ""},
		},
		{
			name:              "finds webp encoder",
			accept:            "image/webp",
			expectEncoderType: &EncoderType{"webp", ""},
		},
		// {
		// 	name:          "finds avif encoder",
		// 	accept:        "image/avif",
		// 	expectEncoder: avif.Encode,
		// },
		{
			name:              "returns first matching encoder",
			accept:            "image/jpeg, image/webp",
			expectEncoderType: &EncoderType{"jpeg", ""},
		},
		// {
		// 	name:              "handles encoder parameter for webp",
		// 	accept:            "image/webp;encoder=chai",
		// 	expectEncoderType: &EncoderType{"webp", "chai"},
		// },
		{
			name:              "handles encoder parameter for webp - jack",
			accept:            "image/webp;encoder=jack",
			expectEncoderType: &EncoderType{"webp", "jack"},
		},
		{
			name:              "handles encoder parameter for webp - jackdyn",
			accept:            "image/webp;encoder=jackdyn",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
		},
		{
			name:              "handles encoder parameter for webp - jacktra",
			accept:            "image/webp;encoder=jacktra",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
		},
		{
			name:              "returns nil for unknown encoder with parameter",
			accept:            "image/jpeg;encoder=unknown",
			expectEncoderType: nil,
		},
		{
			name:              "empty accept header uses jpeg",
			accept:            "",
			expectEncoderType: &EncoderType{"*", ""},
		},
		{
			name:              "accept all types uses jpeg",
			accept:            "*/*",
			expectEncoderType: &EncoderType{"*", ""},
		},
		{
			name:              "returns nil for unsupported type",
			accept:            "image/gif",
			expectEncoderType: nil,
		},
		{
			name:              "respects quality preference ordering",
			accept:            "image/png;q=0.2, image/jpeg;q=1.0",
			expectEncoderType: &EncoderType{"jpeg", ""},
		},
		{
			name:              "selects highest quality supported format",
			accept:            "image/gif;q=0.9, image/jpeg;q=0.7, image/png;q=0.8",
			expectEncoderType: &EncoderType{"png", ""},
		},
		// {
		// 	name:          "real world browser accept header",
		// 	accept:        "image/avif, image/webp, image/png, image/svg+xml, image/*;q=0.8, */*;q=0.5",
		// 	expectEncoder: avif.Encode,
		// },
		// {
		// 	name:          "modern browser with quality preferences",
		// 	accept:        "image/avif;q=0.9, image/webp;q=0.8, image/jpeg;q=0.6, image/*;q=0.4",
		// 	expectEncoder: avif.Encode,
		// },
		{
			name:              "jpeg with quality parameter",
			accept:            "image/jpeg;quality=100",
			expectEncoderType: &EncoderType{"jpeg", ""},
			expectQuality:     100,
		},
		{
			name:              "jpeg with quality 90",
			accept:            "image/jpeg;quality=90",
			expectEncoderType: &EncoderType{"jpeg", ""},
			expectQuality:     90,
		},
		{
			name:              "jpeg with quality 80",
			accept:            "image/jpeg;quality=80",
			expectEncoderType: &EncoderType{"jpeg", ""},
			expectQuality:     80,
		},
		{
			name:              "jpeg with quality 70",
			accept:            "image/jpeg;quality=70",
			expectEncoderType: &EncoderType{"jpeg", ""},
			expectQuality:     70,
		},
		{
			name:              "jpeg with quality 60",
			accept:            "image/jpeg;quality=60",
			expectEncoderType: &EncoderType{"jpeg", ""},
			expectQuality:     60,
		},
		{
			name:              "jpeg with quality 50",
			accept:            "image/jpeg;quality=50",
			expectEncoderType: &EncoderType{"jpeg", ""},
			expectQuality:     50,
		},
		{
			name:              "webp with unknown encoder",
			accept:            "image/webp;encoder=HugoSmits86",
			expectEncoderType: nil,
		},
		// {
		// 	name:          "webp  with quality 100",
		// 	accept:        "image/webp;encoder=chai;quality=100",
		// 	expectEncoder: webpchai.Encode,
		// 	expectQuality: 100,
		// },
		// {
		// 	name:          "webp chai with quality 90",
		// 	accept:        "image/webp;encoder=chai;quality=90",
		// 	expectEncoder: webpchai.Encode,
		// 	expectQuality: 90,
		// },
		// {
		// 	name:          "webp chai with quality 80",
		// 	accept:        "image/webp;encoder=chai;quality=80",
		// 	expectEncoder: webpchai.Encode,
		// 	expectQuality: 80,
		// },
		// {
		// 	name:          "webp chai with quality 70",
		// 	accept:        "image/webp;encoder=chai;quality=70",
		// 	expectEncoder: webpchai.Encode,
		// 	expectQuality: 70,
		// },
		// {
		// 	name:          "webp chai with quality 60",
		// 	accept:        "image/webp;encoder=chai;quality=60",
		// 	expectEncoder: webpchai.Encode,
		// 	expectQuality: 60,
		// },
		// {
		// 	name:          "webp chai with quality 50",
		// 	accept:        "image/webp;encoder=chai;quality=50",
		// 	expectEncoder: webpchai.Encode,
		// 	expectQuality: 50,
		// },
		{
			name:              "webp jackdyn with mem and quality 100",
			accept:            "image/webp;encoder=jackdyn;quality=100",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
			expectQuality:     100,
		},
		{
			name:              "webp jackdyn with mem and quality 90",
			accept:            "image/webp;encoder=jackdyn;quality=90",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
			expectQuality:     90,
		},
		{
			name:              "webp jackdyn with mem and quality 80",
			accept:            "image/webp;encoder=jackdyn;quality=80",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
			expectQuality:     80,
		},
		{
			name:              "webp jackdyn with mem and quality 70",
			accept:            "image/webp;encoder=jackdyn;quality=70",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
			expectQuality:     70,
		},
		{
			name:              "webp jackdyn with mem and quality 60",
			accept:            "image/webp;encoder=jackdyn;quality=60",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
			expectQuality:     60,
		},
		{
			name:              "webp jackdyn with mem and quality 50",
			accept:            "image/webp;encoder=jackdyn;quality=50",
			expectEncoderType: &EncoderType{"webp", "jackdyn"},
			expectQuality:     50,
		},
		{
			name:              "webp jacktra with mem and quality 100",
			accept:            "image/webp;encoder=jacktra;quality=100",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
			expectQuality:     100,
		},
		{
			name:              "webp jacktra with mem and quality 90",
			accept:            "image/webp;encoder=jacktra;quality=90",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
			expectQuality:     90,
		},
		{
			name:              "webp jacktra with mem and quality 80",
			accept:            "image/webp;encoder=jacktra;quality=80",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
			expectQuality:     80,
		},
		{
			name:              "webp jacktra with mem and quality 70",
			accept:            "image/webp;encoder=jacktra;quality=70",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
			expectQuality:     70,
		},
		{
			name:              "webp jacktra with mem and quality 60",
			accept:            "image/webp;encoder=jacktra;quality=60",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
			expectQuality:     60,
		},
		{
			name:              "webp jacktra with mem and quality 50",
			accept:            "image/webp;encoder=jacktra;quality=50",
			expectEncoderType: &EncoderType{"webp", "jacktra"},
			expectQuality:     50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges, err := ParseAccept(tt.accept)
			if err != nil {
				t.Fatalf("unexpected error parsing accept header: %v", err)
			}
			if len(ranges) == 0 {
				if tt.expectEncoderType != nil {
					t.Fatal("expected encoder but got none")
				}
				// No ranges and no expected encoder is a valid case
				return
			}
			encoder, mr, ok := ranges.FirstSupported()

			if tt.expectQuality != 0 {
				quality := mr.QualityParam()
				if quality != tt.expectQuality {
					t.Errorf("expected quality %d, got %d", tt.expectQuality, quality)
				}
			}

			if tt.expectEncoderType != nil {
				if !ok {
					t.Error("expected encoder but got none")
					return
				}
				if encoder.Func == nil {
					t.Error("expected non-nil encoder function")
					return
				}
				// Compare encoder types directly
				if encoder.Type != *tt.expectEncoderType {
					t.Errorf("expected encoder type %+v, got %+v", *tt.expectEncoderType, encoder.Type)
				}
				// Verify media range subtype matches encoder subtype
				if mr.Subtype != tt.expectEncoderType.Subtype {
					t.Errorf("expected media range subtype %q, got %q", tt.expectEncoderType.Subtype, mr.Subtype)
				}
			} else {
				if ok {
					t.Error("expected no encoder but got one")
				}
			}
		})
	}
}
