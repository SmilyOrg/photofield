package io

import (
	"context"
	"fmt"
	"image"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

type AspectRatioFit int32

func (f *AspectRatioFit) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	switch strings.ToUpper(s) {
	default:
		*f = OriginalSize
	case "INSIDE":
		*f = FitInside
	case "OUTSIDE":
		*f = FitOutside
	}
	return nil
}

const (
	OriginalSize AspectRatioFit = iota + 1
	FitOutside
	FitInside
)

type Orientation int8

// All orientations are counter-clockwise, so to display the photo as intended,
// you need to rotate it clockwise by the specified degrees.
//
// For mirror orientations, the image is flipped horizontally or vertically,
// and then rotated by the specified degrees.
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

func (orientation Orientation) SwapsDimensions() bool {
	switch orientation {
	case Normal:
		return false
	case MirrorHorizontal:
		return false
	case Rotate180:
		return false
	case MirrorVertical:
		return false
	case MirrorHorizontalRotate270:
		return true
	case Rotate90:
		return true
	case MirrorHorizontalRotate90:
		return true
	case Rotate270:
		return true
	default:
		return false
	}
}

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
	FromCache   bool
	Error       error
}

type Source interface {
	Name() string
	DisplayName() string
	Ext() string
	Size(original Size) Size
	Rotate() bool
	GetDurationEstimate(original Size) time.Duration
	Exists(ctx context.Context, id ImageId, path string) bool
	Get(ctx context.Context, id ImageId, path string) Result
	Close() error
}

type GetterWithSize interface {
	GetWithSize(ctx context.Context, id ImageId, path string, original Size) Result
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

type ReadDecoderSource interface {
	Source
	ReadDecoder
}

type Sources []Source

// Original
// var UnderdrawPenaltyMultiplier = 15.
// var SizeCostMultiplier = 0.00001
// var DurationCostMultiplier = 0.003

// Optimized for 0.9 max width ratio + square duration
var DefaultOptions = Options{
	UnderdrawPenaltyMultiplier: 59.851585,
	SizeCostMultiplier:         0.000281,
	DurationCostMultiplier:     0.011857,
}

type Options struct {
	UnderdrawPenaltyMultiplier float64
	SizeCostMultiplier         float64
	DurationCostMultiplier     float64
}

func SizeCost(source Size, original Size, target Size, opts Options) (cost float64, area int64) {
	if source.X == 0 && source.Y == 0 {
		source = target
	}
	area = source.Area()
	targetArea := target.Area()
	diff := float64(targetArea) - float64(area)
	if targetArea > area {
		diff *= opts.UnderdrawPenaltyMultiplier
	}
	cost = diff * diff * opts.SizeCostMultiplier
	return
}

func DurationCost(dur time.Duration, opts Options) float64 {
	us := float64(dur.Microseconds())
	return us * us * opts.DurationCostMultiplier
}

func (sources Sources) EstimateCostWithOpts(original Size, target Size, opts Options) SourceCosts {
	costs := make([]SourceCost, len(sources))
	for i := range sources {
		s := sources[i]
		sizecost, sarea := SizeCost(s.Size(original), original, target, opts)
		dur := s.GetDurationEstimate(original)
		durcost := DurationCost(dur, opts)
		cost := sizecost + durcost
		costs[i] = SourceCost{
			Source:            s,
			EstimatedArea:     sarea,
			EstimatedDuration: dur,
			SizeCost:          sizecost,
			DurationCost:      durcost,
			Cost:              cost,
		}
	}
	return costs
}

func (sources Sources) EstimateCost(original Size, target Size) SourceCosts {
	return sources.EstimateCostWithOpts(original, target, DefaultOptions)
}

func (sources Sources) Close() {
	for _, s := range sources {
		err := s.Close()
		if err != nil {
			panic(err)
		}
	}
}

type SourceCost struct {
	Source
	EstimatedArea     int64
	EstimatedDuration time.Duration
	SizeCost          float64
	DurationCost      float64
	Cost              float64
}

type SourceCosts []SourceCost

func (costs SourceCosts) Sort() {
	sort.Slice(costs, func(i, j int) bool {
		a := costs[i]
		b := costs[j]
		return a.Cost < b.Cost
	})
}

func (costs SourceCosts) SortSize() {
	sort.Slice(costs, func(i, j int) bool {
		a := costs[i]
		b := costs[j]
		return a.SizeCost < b.SizeCost
	})
}
