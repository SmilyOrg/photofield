// BEGIN: 8f7e2d3b4c5a
package fs

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestHashFile(t *testing.T) {
	// Create a temporary file with some content
	tmpfile, err := ioutil.TempFile("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Compute the hash of the temporary file
	hash, err := HashFile(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Verify that the hash is correct
	expectedHash := uint64(5020219685658847592)
	if hash != expectedHash {
		t.Errorf("HashFile(%q) = %d, want %d", tmpfile.Name(), hash, expectedHash)
	}
}

func BenchmarkHashFile(b *testing.B) {
	// Create a temporary file with some content
	tmpfile, err := ioutil.TempFile("", "testfile")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write([]byte("hello world")); err != nil {
		b.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatal(err)
	}

	// Benchmark the HashFile function
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := HashFile(tmpfile.Name()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBufferSizes(b *testing.B) {
	// Create a temporary file with some content
	tmpfile, err := ioutil.TempFile("", "testfile")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	data := make([]byte, 4*1024*1024)
	if _, err := rand.Read(data); err != nil {
		log.Fatal(err)
	}

	if _, err := tmpfile.Write(data); err != nil {
		b.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatal(err)
	}

	s, err := os.Stat(tmpfile.Name())
	if err != nil {
		b.Fatal(err)
	}
	filesize := s.Size()

	// Benchmark the HashFile function with different buffer sizes

	for i := 8; i < 26; i++ {
		size := 1 << uint(i)
		b.Run(fmt.Sprintf("buffer=%d", size), func(b *testing.B) {
			b.SetBytes(int64(filesize))
			// BufferSize = size
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := HashFile(tmpfile.Name()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
