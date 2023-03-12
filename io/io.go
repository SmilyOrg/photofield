package io

import (
	"context"
	"fmt"
	"image"
	"io"
	"math"
	"sort"
	"time"
)

type AspectRatioFit int32

const (
	FitOutside   AspectRatioFit = iota
	FitInside    AspectRatioFit = iota
	OriginalSize AspectRatioFit = iota
)

type Orientation int8

// All rotations are counter-clockwise
const (
	Normal                    Orientation = 1
	MirrorHorizontal          Orientation = 2
	Rotate180                 Orientation = 3
	MirrorVertical            Orientation = 4
	MirrorHorizontalRotate270 Orientation = 5
	Rotate90                  Orientation = 6
	MirrorHorizontalRotate90  Orientation = 7
	Rotate270                 Orientation = 8
	SourceInfoOrientation     Orientation = 127
)

type ImageId uint32

type Size image.Point

func (s Size) String() string {
	return fmt.Sprintf("%d x %d", s.X, s.Y)
}

func (s Size) Area() int64 {
	return int64(s.X) * int64(s.Y)
}

func (s Size) Fit(original Size, fit AspectRatioFit) Size {
	if fit == OriginalSize {
		return original
	}
	tw, th := float64(s.X), float64(s.Y)
	ar := tw / th
	ow, oh := float64(original.X), float64(original.Y)
	oar := ow / oh
	switch fit {
	case FitInside:
		if ar < oar {
			th = tw / oar
		} else {
			tw = th * oar
		}
	case FitOutside:
		if ar > oar {
			th = tw / oar
		} else {
			tw = th * oar
		}
	}
	return Size{
		X: int(math.Round(tw)),
		Y: int(math.Round(th)),
	}
}

type Result struct {
	Image       image.Image
	Orientation Orientation
	Error       error
}

type Source interface {
	Name() string
	Ext() string
	Size(original Size) Size
	Rotate() bool
	GetDurationEstimate(original Size) time.Duration
	Exists(ctx context.Context, id ImageId, path string) bool
	Get(ctx context.Context, id ImageId, path string) Result
}

type Sink interface {
	Set(ctx context.Context, id ImageId, path string, r Result) bool
}

type Reader interface {
	Reader(ctx context.Context, id ImageId, path string, fn func(r io.ReadSeeker, err error))
}

type Decoder interface {
	Decode(ctx context.Context, r io.Reader) Result
}

type ReadDecoder interface {
	Reader
	Decoder
}

type Sources []Source

func (sources Sources) EstimateCost(original Size, target Size) SourceCosts {
	targetArea := target.Area()
	costs := make([]SourceCost, len(sources))
	for i := range sources {
		s := sources[i]
		ssize := s.Size(original)
		if ssize.X == 0 && ssize.Y == 0 {
			ssize = target
		}
		sarea := ssize.Area()
		sizecost := math.Abs(float64(targetArea)-float64(sarea)) * 0.001
		if targetArea > sarea {
			// areacost = math.Sqrt(float64(targetArea)-float64(sarea)) * 3
			// areacost = math.Sqrt(float64(targetArea)-float64(sarea)) * 3
			sizecost *= 7
		}
		// dx := float64(target.X - ssize.X)
		// dy := float64(target.Y - ssize.Y)
		// sizecost := math.Sqrt(dx*dx + dy*dy)
		dur := s.GetDurationEstimate(original)
		durcost := math.Pow(float64(dur.Microseconds()), 1) * 0.003
		// durcost := float64(dur.Microseconds()) * 0.001
		cost := sizecost + durcost
		// fmt.Printf("%4d %30s %12s %12s %12s %12d %12f %10s %12f %12f\n", i, s.Name(), original, target, ssize, sarea, sizecost, dur, durcost, cost)
		costs[i] = SourceCost{
			Source: s,
			Cost:   cost,
		}
	}
	return costs
}

type SourceCost struct {
	Source
	Cost float64
}

type SourceCosts []SourceCost

func (costs SourceCosts) Sort() {
	sort.Slice(costs, func(i, j int) bool {
		a := costs[i]
		b := costs[j]
		return a.Cost < b.Cost
	})
}
