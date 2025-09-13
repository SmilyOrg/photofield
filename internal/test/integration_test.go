package test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhotoGenerationWithPool(t *testing.T) {
	// Create a temporary directory for test output
	tempDir := t.TempDir()

	// Define a simple test dataset
	dataset := TestDataset{
		Name: "pool-test",
		Images: []ImageSpec{
			{Width: 100, Height: 100},
			{Width: 200, Height: 150},
			{Width: 100, Height: 100}, // Same size as first - should reuse pool
		},
		Samples: 2,
		Seed:    12345,
	}

	// Generate the test images
	images, err := GenerateTestDataset(tempDir, dataset)
	if err != nil {
		t.Fatalf("Failed to generate test dataset: %v", err)
	}

	// Verify we got the expected number of images
	expectedCount := len(dataset.Images) * dataset.Samples
	if len(images) != expectedCount {
		t.Errorf("Expected %d images, got %d", expectedCount, len(images))
	}

	// Verify all images were created and have correct specs
	for i, img := range images {
		// Check that file exists
		if _, err := os.Stat(img.Path); os.IsNotExist(err) {
			t.Errorf("Image file does not exist: %s", img.Path)
			continue
		}

		// Check that spec matches expected
		specIndex := i / dataset.Samples
		expectedSpec := dataset.Images[specIndex]
		if img.Spec.Width != expectedSpec.Width || img.Spec.Height != expectedSpec.Height {
			t.Errorf("Image %d has wrong spec: got %dx%d, want %dx%d",
				i, img.Spec.Width, img.Spec.Height, expectedSpec.Width, expectedSpec.Height)
		}

		// Check that path is in the correct directory
		expectedDir := filepath.Join(tempDir, dataset.Name)
		if filepath.Dir(img.Path) != expectedDir {
			t.Errorf("Image %d has wrong directory: got %s, want %s",
				i, filepath.Dir(img.Path), expectedDir)
		}
	}
}
