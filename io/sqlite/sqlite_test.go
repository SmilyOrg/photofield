package sqlite

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path"
	"photofield/internal/test"
	"photofield/io"
	"photofield/io/goimage"
	"strconv"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	dataset := test.TestDataset{
		Name:    "sqlite-roundtrip",
		Seed:    456,
		Samples: 1,
		Images: []test.ImageSpec{
			{Width: 256, Height: 171},
		},
	}
	images, err := test.GenerateTestDataset("../../testdata", dataset)
	if err != nil {
		t.Fatalf("failed to generate test dataset: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	image := images[0]

	p := image.Path
	bytes, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	dbPath := path.Join(os.TempDir(), "photofield.thumbs.db")
	defer os.Remove(dbPath)
	s := New(dbPath)

	id := uint32(1)

	s.Write(id, bytes)
	s.Flush() // Wait for the write to complete
	r := s.Get(context.Background(), io.ImageId(id), p)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	b := r.Image.Bounds()
	if b.Dx() != 256 || b.Dy() != 171 {
		t.Errorf("unexpected size %d x %d", b.Dx(), b.Dy())
	}
}

// colorsEqual compares two colors for equality
func colorsEqual(c1, c2 color.Color) bool {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// TestRotateImageIfNeeded tests the rotations of an image with distinctive corners
func TestRotateImageIfNeeded(t *testing.T) {
	// Create a distinctive test pattern: a 2x2 image with different colors
	// This allows us to verify exact rotation behavior
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	// Set up a pattern:
	// R G
	// B Y
	red := color.RGBA{255, 0, 0, 255}
	green := color.RGBA{0, 255, 0, 255}
	blue := color.RGBA{0, 0, 255, 255}
	yellow := color.RGBA{255, 255, 0, 255}

	img.Set(0, 0, red)    // Top-left: Red
	img.Set(1, 0, green)  // Top-right: Green
	img.Set(0, 1, blue)   // Bottom-left: Blue
	img.Set(1, 1, yellow) // Bottom-right: Yellow

	tests := []struct {
		name        string
		orientation io.Orientation
		expectedTL  color.Color // expected top-left color after transformation
		expectedTR  color.Color // expected top-right color after transformation
		expectedBL  color.Color // expected bottom-left color after transformation
		expectedBR  color.Color // expected bottom-right color after transformation
	}{
		{
			name:        "Normal - no change",
			orientation: io.Normal,
			expectedTL:  red,
			expectedTR:  green,
			expectedBL:  blue,
			expectedBR:  yellow,
		},
		{
			name:        "MirrorHorizontal - flip horizontally",
			orientation: io.MirrorHorizontal,
			expectedTL:  green,
			expectedTR:  red,
			expectedBL:  yellow,
			expectedBR:  blue,
		},
		{
			name:        "Rotate180 - rotate 180 degrees",
			orientation: io.Rotate180,
			expectedTL:  yellow,
			expectedTR:  blue,
			expectedBL:  green,
			expectedBR:  red,
		},
		{
			name:        "MirrorVertical - flip vertically",
			orientation: io.MirrorVertical,
			expectedTL:  blue,
			expectedTR:  yellow,
			expectedBL:  red,
			expectedBR:  green,
		},
		{
			name:        "MirrorHorizontalRotate270",
			orientation: io.MirrorHorizontalRotate270,
			expectedTL:  red,
			expectedTR:  blue,
			expectedBL:  green,
			expectedBR:  yellow,
		},
		{
			name:        "Rotate90",
			orientation: io.Rotate90,
			expectedTL:  blue,
			expectedTR:  red,
			expectedBL:  yellow,
			expectedBR:  green,
		},
		{
			name:        "MirrorHorizontalRotate90",
			orientation: io.MirrorHorizontalRotate90,
			expectedTL:  yellow,
			expectedTR:  green,
			expectedBL:  blue,
			expectedBR:  red,
		},
		{
			name:        "Rotate270",
			orientation: io.Rotate270,
			expectedTL:  green,
			expectedTR:  yellow,
			expectedBL:  red,
			expectedBR:  blue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rotateImageIfNeeded(img, tt.orientation)

			// Basic sanity checks
			if result == nil {
				t.Fatal("rotateImageIfNeeded returned nil")
			}

			bounds := result.Bounds()
			if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
				t.Errorf("invalid bounds: %v", bounds)
			}

			// For Normal orientation, should be the same instance
			if tt.orientation == io.Normal && result != img {
				t.Error("Normal orientation should return the same image instance")
			}

			// For other orientations, should be different instance (unless the rotate library optimizes)
			if tt.orientation != io.Normal && result == img {
				// This might be OK if the rotation library optimizes, but let's log it
				t.Logf("Warning: %s returned same instance (may be optimized)", tt.name)
			}

			// Check specific color patterns
			actualTL := result.At(bounds.Min.X, bounds.Min.Y)
			actualTR := result.At(bounds.Max.X-1, bounds.Min.Y)
			actualBL := result.At(bounds.Min.X, bounds.Max.Y-1)
			actualBR := result.At(bounds.Max.X-1, bounds.Max.Y-1)

			if !colorsEqual(actualTL, tt.expectedTL) {
				t.Errorf("%s: top-left color mismatch, expected %v, got %v",
					tt.name, tt.expectedTL, actualTL)
			}
			if !colorsEqual(actualTR, tt.expectedTR) {
				t.Errorf("%s: top-right color mismatch, expected %v, got %v",
					tt.name, tt.expectedTR, actualTR)
			}
			if !colorsEqual(actualBL, tt.expectedBL) {
				t.Errorf("%s: bottom-left color mismatch, expected %v, got %v",
					tt.name, tt.expectedBL, actualBL)
			}
			if !colorsEqual(actualBR, tt.expectedBR) {
				t.Errorf("%s: bottom-right color mismatch, expected %v, got %v",
					tt.name, tt.expectedBR, actualBR)
			}
		})
	}
}

func TestEncodeOrientation(t *testing.T) {
	dataset := test.TestDataset{
		Name:    "sqlite-encode-orientation",
		Seed:    123,
		Samples: 1,
		Images: []test.ImageSpec{
			{Width: 256, Height: 170}, // No tag
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "1"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "2"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "3"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "4"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "5"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "6"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "7"}},
			{Width: 256, Height: 170, ExifTags: map[string]string{"Orientation#": "8"}},
			{Width: 170, Height: 256}, // No tag
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "1"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "2"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "3"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "4"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "5"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "6"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "7"}},
			{Width: 170, Height: 256, ExifTags: map[string]string{"Orientation#": "8"}},
		},
	}
	images, err := test.GenerateTestDataset("../../testdata", dataset)
	if err != nil {
		t.Fatalf("failed to generate test dataset: %v", err)
	}
	src := Source{
		encodePng: true,
	}
	for _, img := range images {
		img := img // capture loop variable
		t.Run(img.Name, func(t *testing.T) {
			// t.Parallel()

			ctx := context.Background()
			r := goimage.Image{}.Get(ctx, 0, img.Path)
			if r.Image == nil {
				t.Errorf("image not found: %s", img.Path)
				return
			}
			if r.Error != nil {
				t.Errorf("image load error: %s", img.Path)
				return
			}

			orientationTag := img.Spec.ExifTags["Orientation#"]
			orientationInt := 1
			if orientationTag != "" {
				orientationInt, err = strconv.Atoi(orientationTag)
				if err != nil {
					t.Errorf("failed to parse orientation tag %q: %v", orientationTag, err)
					return
				}
			}
			orientation := io.Orientation(orientationInt)
			r.Orientation = orientation

			w := &bytes.Buffer{}
			ok := src.Encode(ctx, r, w)
			if !ok {
				t.Errorf("encoding failed: %s", img.Path)
				return
			}

			original := r.Image

			// Save to file for debugging
			debugPath := img.Path + ".encoded.png"
			if err := os.WriteFile(debugPath, w.Bytes(), 0644); err != nil {
				t.Errorf("failed to write debug file %s: %v", debugPath, err)
			}

			encoded, err := png.Decode(w)
			if err != nil {
				t.Errorf("failed to decode PNG: %s", img.Path)
				return
			}

			ow, oh := original.Bounds().Dx(), original.Bounds().Dy()
			ew, eh := encoded.Bounds().Dx(), encoded.Bounds().Dy()

			// Check corner pixels to verify orientation
			otl := original.At(0, 0)       // Top-left corner
			otr := original.At(ow-1, 0)    // Top-right corner
			obl := original.At(0, oh-1)    // Bottom-left corner
			obr := original.At(ow-1, oh-1) // Bottom-right corner
			etl := encoded.At(0, 0)        // Top-left corner
			etr := encoded.At(ew-1, 0)     // Top-right corner
			ebl := encoded.At(0, eh-1)     // Bottom-left corner
			ebr := encoded.At(ew-1, eh-1)  // Bottom-right corner

			// For rotated images (90/270 degrees), dimensions should be swapped
			shouldBeRotated := orientation == io.Rotate90 || orientation == io.Rotate270 ||
				orientation == io.MirrorHorizontalRotate270 || orientation == io.MirrorHorizontalRotate90

			tl, tr, bl, br := otl, otr, obl, obr
			if shouldBeRotated {
				if (ew > eh) == (ow > oh) {
					t.Errorf("image %s appears not to be rotated: thumb=%dx%d, orig=%dx%d, orientation=%s",
						img.Path, ew, eh, ow, oh, orientationTag)
				}
			} else {
				if (ew > eh) != (ow > oh) {
					t.Errorf("image %s appears incorrectly rotated: thumb=%dx%d, orig=%dx%d, orientation=%s",
						img.Path, ew, eh, ow, oh, orientationTag)
				}
			}

			tl, tr, bl, br = test.TransformCornersByNumber(
				otl, otr, obl, obr,
				orientationInt,
			)

			if !test.ColorsSimilar(etl, tl, 4) || !test.ColorsSimilar(etr, tr, 4) ||
				!test.ColorsSimilar(ebl, bl, 4) || !test.ColorsSimilar(ebr, br, 4) {
				t.Errorf("image %s encoding has incorrect %s orientation", img.Path, orientationTag)
				// Show the difference in corners visually per corner next to each other
				// Print each corner in a grid format and highlight differences
				t.Log("original  | expected  | encoded   | diff")
				t.Logf("%4d %4d | %4d %4d | %4d %4d | %4d %4d",
					test.ColorToInt(otl), test.ColorToInt(otr),
					test.ColorToInt(tl), test.ColorToInt(tr),
					test.ColorToInt(etl), test.ColorToInt(etr),
					test.ColorDiff(tl, etl), test.ColorDiff(tr, etr))
				t.Logf("%4d %4d | %4d %4d | %4d %4d | %4d %4d",
					test.ColorToInt(obl), test.ColorToInt(obr),
					test.ColorToInt(bl), test.ColorToInt(br),
					test.ColorToInt(ebl), test.ColorToInt(ebr),
					test.ColorDiff(bl, ebl), test.ColorDiff(br, ebr))
			}
		})
	}
}
