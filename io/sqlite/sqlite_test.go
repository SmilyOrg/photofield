package sqlite

import (
	"context"
	"embed"
	"os"
	"path"
	"photofield/io"
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
	r := s.Get(context.Background(), io.ImageId(id), p)
	if r.Error != nil {
		t.Fatal(r.Error)
	}
	b := r.Image.Bounds()
	if b.Dx() != 256 || b.Dy() != 171 {
		t.Errorf("unexpected size %d x %d", b.Dx(), b.Dy())
	}
}
