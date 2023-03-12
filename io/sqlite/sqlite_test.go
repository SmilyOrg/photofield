package sqlite

import (
	"context"
	"embed"
	"os"
	"path"
	"testing"
)

var dir = "../../../photos/"

func TestRoundtrip(t *testing.T) {
	p := path.Join(dir, "test/P1110220-ffmpeg-256-cjpeg-70.jpg")
	bytes, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	s := New(path.Join(dir, "test/photofield.thumbs.db"), embed.FS{})

	id := uint32(1)

	s.Write(id, bytes)
	img, err := s.Load(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 256 || b.Dy() != 171 {
		t.Errorf("unexpected size %d x %d", b.Dx(), b.Dy())
	}
}
