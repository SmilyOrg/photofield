package fs

import (
	"io"
	"os"

	"github.com/cespare/xxhash/v2"
)

func HashFile(path string) (hash uint64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return HashReader(f)
}

func HashReader(src io.Reader) (hash uint64, err error) {
	d := xxhash.New()
	_, err = io.Copy(d, src)
	if err != nil {
		return 0, err
	}
	return d.Sum64(), nil
}
