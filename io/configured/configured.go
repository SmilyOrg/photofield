package configured

import (
	"context"
	"fmt"
	"photofield/io"
	"time"

	goio "io"

	"github.com/goccy/go-yaml"
)

type Cost struct {
	Time                     Duration `json:"time"`
	TimePerOriginalMegapixel Duration `json:"time_per_original_megapixel"`
	TimePerResizedMegapixel  Duration `json:"time_per_resized_megapixel"`
}

type Duration time.Duration

func (d *Duration) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

func (d Duration) String() string {
	return time.Duration(d).String()
}

type Configured struct {
	NameStr string
	Cost    Cost
	Source  io.Source
}

func New(name string, cost Cost, source io.Source) *Configured {
	c := Configured{
		NameStr: name,
		Cost:    cost,
		Source:  source,
	}
	if c.NameStr == "" {
		c.NameStr = c.Source.Name()
	}
	return &c
}

func (c *Configured) Close() error {
	return c.Source.Close()
}

func (c *Configured) Name() string {
	return c.NameStr
}

func (c *Configured) DisplayName() string {
	return c.Source.DisplayName()
}

func (c *Configured) Ext() string {
	return c.Source.Ext()
}

func (c *Configured) Size(size io.Size) io.Size {
	return c.Source.Size(size)
}

func (c *Configured) GetDurationEstimate(original io.Size) time.Duration {
	resized := c.Size(original)
	t := c.Cost.Time
	tomp := c.Cost.TimePerOriginalMegapixel
	trmp := c.Cost.TimePerResizedMegapixel
	d := Duration(t + (tomp*Duration(original.Area())+trmp*Duration(resized.Area()))/1e6)
	return time.Duration(d)
}

func (c *Configured) Rotate() bool {
	return c.Source.Rotate()
}

func (c *Configured) Exists(ctx context.Context, id io.ImageId, path string) bool {
	return c.Source.Exists(ctx, id, path)
}

func (c *Configured) Get(ctx context.Context, id io.ImageId, path string, original io.Size) io.Result {
	return c.Source.Get(ctx, id, path, original)
}

func (c *Configured) Reader(ctx context.Context, id io.ImageId, path string, fn func(r goio.ReadSeeker, err error)) {
	r, ok := c.Source.(io.Reader)
	if !ok {
		fn(nil, fmt.Errorf("reader not supported by %s", c.Source.Name()))
		return
	}
	r.Reader(ctx, id, path, fn)
}

func (c *Configured) Decode(ctx context.Context, r goio.Reader) io.Result {
	d, ok := c.Source.(io.Decoder)
	if !ok {
		return io.Result{Error: fmt.Errorf("decoder not supported by %s", c.Source.Name())}
	}
	return d.Decode(ctx, r)
}
