package cached

import (
	"context"
	"errors"
	"fmt"
	"photofield/io"
	"photofield/io/ristretto"
	"time"

	goio "io"

	"golang.org/x/sync/singleflight"
)

type Cached struct {
	Source  io.Source
	Cache   ristretto.Ristretto
	loading singleflight.Group
}

func (c *Cached) Close() error {
	return errors.Join(
		c.Source.Close(),
		c.Cache.Close(),
	)
}

func (c *Cached) Name() string {
	return c.Source.Name()
}

func (c *Cached) DisplayName() string {
	return c.Source.DisplayName()
}

func (c *Cached) Ext() string {
	return c.Source.Ext()
}

func (c *Cached) Size(size io.Size) io.Size {
	return c.Source.Size(size)
}

func (c *Cached) GetDurationEstimate(size io.Size) time.Duration {
	return c.Source.GetDurationEstimate(size)
}

func (c *Cached) Rotate() bool {
	return false
}

func (c *Cached) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return c.Source.Exists(ctx, id, path)
}

func (c *Cached) Get(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	r := c.Cache.GetWithName(ctx, id, c.Source.Name())
	// fmt.Printf("%v %v %v\n", id, c.Source.Name(), r.Error)
	if r.Image != nil || r.Error != nil {
		// fmt.Printf("%v %v cache found\n", id, c.Source.Name())
		// println("found in cache")
		r.FromCache = true
		return r
	}
	// r = c.Source.Get(ctx, id, path)
	r = c.load(ctx, id, path, original)
	// fmt.Printf("%v %v cache load end\n", id, c.Source.Name())
	// c.Ristretto.SetWithName(ctx, id, c.Source.Name(), r)
	// fmt.Printf("%v cache set\n", id)
	// println("saved to cache", s)
	return r
}

func (c *Cached) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	r, ok := c.Source.(io.Reader)
	if !ok {
		fn(nil, fmt.Errorf("reader not supported by %s", c.Source.Name()))
		return
	}
	r.Reader(ctx, id, path, fn)
}

func (c *Cached) load(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	key := fmt.Sprintf("%d", id)
	// fmt.Printf("%v cache load begin %v\n", id, key)
	ri, _, _ := c.loading.Do(key, func() (interface{}, error) {
		// fmt.Printf("%p %v %s %v cache get begin\n", c, c.Source, c.Source.Name(), id)
		r := c.Source.Get(ctx, id, path, original)
		// fmt.Printf("%p %v %s %v cache get end\n", c, c.Source, c.Source.Name(), id)
		c.Cache.SetWithName(ctx, id, c.Source.Name(), r)
		// fmt.Printf("%v cache set\n", id)
		return r, nil
	})
	// fmt.Printf("%v cache load end %v\n", id, key)
	return ri.(io.Result)
}

func (c *Cached) Set(ctx context.Context, id io.ImageId, path string, r io.Result) bool {
	return false
}
