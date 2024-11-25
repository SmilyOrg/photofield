package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	// Check if arguments are provided
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <file1> <file2> ...", os.Args[0])
	}

	for _, filePath := range os.Args[1:] {
		// Check if file exists and is not a dir
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Printf("Failed to stat %s: %v", filePath, err)
			continue
		}
		if fileInfo.IsDir() {
			continue
		}

		// Compute SHA-256 checksum
		checksum, err := computeSHA256(filePath)
		if err != nil {
			log.Printf("Failed to compute checksum for %s: %v", filePath, err)
			continue
		}

		// Print filename and checksum to stdout
		fmt.Printf("%x  %s\n", checksum, filePath)
	}
}

func computeSHA256(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}

	return hash.Sum(nil), nil
}
