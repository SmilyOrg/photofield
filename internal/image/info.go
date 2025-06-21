package image

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/golang/geo/s1"
	"github.com/golang/geo/s2"
)

type Size = image.Point

type Info struct {
	Width, Height int
	DateTime      time.Time
	Color         uint32
	Orientation   Orientation
	LatLng        s2.LatLng
}

const earthRadiusKm = 6371.01

func NaNLatLng() s2.LatLng {
	return s2.LatLng{Lat: s1.Angle(math.NaN()), Lng: s1.Angle(math.NaN())}
}

func IsNaNLatLng(latlng s2.LatLng) bool {
	return math.IsNaN(float64(latlng.Lat)) || math.IsNaN(float64(latlng.Lng))
}

func IsValidLatLng(latlng s2.LatLng) bool {
	return !IsNaNLatLng(latlng) && latlng.Lat.Radians() != 0 && latlng.Lng.Radians() != 0
}

func AngleToKm(a s1.Angle) float64 {
	return a.Radians() * earthRadiusKm
}

func (info *Info) AspectRatio() float64 {
	if info.Height == 0 {
		return 3 / 2
	}
	return float64(info.Width) / float64(info.Height)
}

func (info *Info) Size() Size {
	return Size{X: info.Width, Y: info.Height}
}

func (info *Info) String() string {
	return fmt.Sprintf("width: %v, height: %v, date: %v, color: %08x, orientation: %s, latlng: %s",
		info.Width,
		info.Height,
		info.DateTime.String(),
		info.Color,
		info.Orientation,
		info.LatLng.String(),
	)
}

func (info *Info) IsZero() bool {
	return info.Width == 0 &&
		info.Height == 0 &&
		info.DateTime.IsZero() &&
		info.Color == 0
}

func (info *Info) GetColor() color.RGBA {
	return color.RGBA{
		A: uint8((info.Color >> 24) & 0xFF),
		R: uint8((info.Color >> 16) & 0xFF),
		G: uint8((info.Color >> 8) & 0xFF),
		B: uint8(info.Color & 0xFF),
	}
}

func (info *Info) SetColorRGBA(color color.RGBA) {
	info.Color = (uint32(color.A&0xFF) << 24) |
		(uint32(color.R&0xFF) << 16) |
		(uint32(color.G&0xFF) << 8) |
		uint32(color.B&0xFF)
}

func (info *Info) SetColorRGB32(r uint32, g uint32, b uint32) {
	info.Color = (uint32(0xFF) << 24) |
		(uint32(r&0xFF) << 16) |
		(uint32(g&0xFF) << 8) |
		uint32(b&0xFF)
}

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
	DummyOrientation          Orientation = -1 // Used for testing, not a valid orientation
)

func (orientation Orientation) IsZero() bool {
	return orientation == 0
}

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

func (orientation Orientation) Rotate270() Orientation {
	switch orientation {
	case Normal:
		return Rotate270
	case MirrorHorizontal:
		return MirrorHorizontalRotate270
	case Rotate180:
		return Rotate90
	case MirrorVertical:
		return MirrorHorizontalRotate90
	case MirrorHorizontalRotate270:
		return MirrorVertical
	case Rotate90:
		return Normal
	case MirrorHorizontalRotate90:
		return MirrorHorizontal
	case Rotate270:
		return Rotate180
	default:
		return orientation
	}
}

func (orientation Orientation) String() string {
	switch orientation {
	case Normal:
		return "Normal (1)"
	case MirrorHorizontal:
		return "MirrorHorizontal (2)"
	case Rotate180:
		return "Rotate180 (3)"
	case MirrorVertical:
		return "MirrorVertical (4)"
	case MirrorHorizontalRotate270:
		return "MirrorHorizontalRotate270 (5)"
	case Rotate90:
		return "Rotate90 (6)"
	case MirrorHorizontalRotate90:
		return "MirrorHorizontalRotate90 (7)"
	case Rotate270:
		return "Rotate270 (8)"
	default:
		return fmt.Sprintf("Unknown (%d)", orientation)
	}
}
