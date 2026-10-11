package djpeg

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// countOwnZombies returns how many of this process's children are zombies.
//
// Linux-only, which is why the test skips elsewhere: reaping is only observable
// through /proc, and this is the behaviour that actually broke in production -
// a counted assertion is worth more here than a mock.
func countOwnZombies(t *testing.T) int {
	t.Helper()
	me := strconv.Itoa(os.Getpid())
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Skipf("cannot read /proc: %v", err)
	}
	n := 0
	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		b, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		// "pid (comm) state ppid ..." - comm may contain spaces, so split after ')'
		i := strings.LastIndex(string(b), ")")
		if i < 0 {
			continue
		}
		f := strings.Fields(string(b)[i+1:])
		if len(f) >= 2 && f[0] == "Z" && f[1] == me {
			n++
		}
	}
	return n
}

// A file djpeg cannot decode must not leak its child process.
//
// Regression test: run() used to return as soon as bmp.Decode failed, skipping
// cmd.Wait(), so every undecodable file left a zombie behind. With enough of
// them the container hits its pids limit and no further fork/exec succeeds.
func TestRunReapsChildOnDecodeFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("zombie reaping is only observable via /proc")
	}
	path := FindPath()
	if path == "" {
		t.Skip("djpeg not installed")
	}

	// Not a JPEG, so djpeg exits non-zero and bmp.Decode fails - the path that leaked.
	dir := t.TempDir()
	bad := filepath.Join(dir, "not-really.jpg")
	if err := os.WriteFile(bad, []byte("this is plainly not a JPEG"), 0o644); err != nil {
		t.Fatal(err)
	}

	o := Djpeg{Path: path, Width: 256, Height: 256}

	before := countOwnZombies(t)
	const attempts = 20
	for i := 0; i < attempts; i++ {
		if _, err := o.run(context.Background(), bad, "1/8"); err == nil {
			t.Fatal("expected a decode error for a non-JPEG file")
		}
	}
	after := countOwnZombies(t)

	if after > before {
		t.Fatalf("leaked %d zombie(s) over %d failed decodes (before=%d after=%d)",
			after-before, attempts, before, after)
	}
}

// The success path must keep working, and must also leave nothing behind.
func TestRunReapsChildOnSuccess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("zombie reaping is only observable via /proc")
	}
	path := FindPath()
	if path == "" {
		t.Skip("djpeg not installed")
	}
	// Encoded here rather than taken from testdata, so the test is self-contained.
	jpg := filepath.Join(t.TempDir(), "sample.jpg")
	f, err := os.Create(jpg)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 128, 255})
		}
	}
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	f.Close()

	o := Djpeg{Path: path, Width: 256, Height: 256}
	before := countOwnZombies(t)
	got, err := o.run(context.Background(), jpg, "1/8")
	if err != nil {
		t.Fatalf("decoding a valid jpeg failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected an image")
	}
	if after := countOwnZombies(t); after > before {
		t.Fatalf("leaked %d zombie(s) on the success path", after-before)
	}
}
