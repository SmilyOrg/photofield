package filtered

import (
	"context"
	"fmt"
	"path/filepath"
	"photofield/io"
	"runtime/trace"
	"strings"
	"time"

	goio "io"
)

type Filtered struct {
	Source     io.Source
	Extensions []string
}

func (f *Filtered) Close() error {
	return f.Source.Close()
}

func (f *Filtered) Name() string {
	return f.Source.Name()
}

func (f *Filtered) DisplayName() string {
	return f.Source.DisplayName()
}

func (f *Filtered) Ext() string {
	return f.Source.Ext()
}

func (f *Filtered) Size(size io.Size) io.Size {
	return f.Source.Size(size)
}

func (f *Filtered) GetDurationEstimate(size io.Size) time.Duration {
	return f.Source.GetDurationEstimate(size)
}

func (f *Filtered) Rotate() bool {
	return f.Source.Rotate()
}

func (f *Filtered) SupportsExtension(path string) bool {
	if len(f.Extensions) == 0 {
		return true
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range f.Extensions {
		if ext == e {
			return true
		}
	}
	return false
}

func (f *Filtered) Exists(ctx context.Context, id io.ImageId, path string) bool {
	if !f.SupportsExtension(path) {
		return false
	}
	return f.Source.Exists(ctx, id, path)
}

func (f *Filtered) Get(ctx context.Context, id io.ImageId, path string) io.Result {
	defer trace.StartRegion(ctx, "filtered.Get").End()
	if !f.SupportsExtension(path) {
		return io.Result{Error: fmt.Errorf("extension not supported")}
	}
	return f.Source.Get(ctx, id, path)
}

func (f *Filtered) GetWithSize(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	defer trace.StartRegion(ctx, "filtered.GetWithSize").End()
	if !f.SupportsExtension(path) {
		return io.Result{Error: fmt.Errorf("extension not supported")}
	}
	if r, ok := f.Source.(io.GetterWithSize); ok {
		return r.GetWithSize(ctx, id, path, original)
	}
	return io.Result{Error: fmt.Errorf("GetWithSize not supported by %s", f.Source.Name())}
}

func (f *Filtered) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	if !f.SupportsExtension(path) {
		fn(nil, fmt.Errorf("extension not supported"))
		return
	}
	r, ok := f.Source.(io.Reader)
	if !ok {
		fn(nil, fmt.Errorf("reader not supported by %s", f.Source.Name()))
		return
	}
	r.Reader(ctx, id, path, fn)
}

func (f *Filtered) Decode(ctx context.Context, r goio.Reader) io.Result {
	d, ok := f.Source.(io.Decoder)
	if !ok {
		return io.Result{Error: fmt.Errorf("decoder not supported by %s", f.Source.Name())}
	}
	return d.Decode(ctx, r)
}
