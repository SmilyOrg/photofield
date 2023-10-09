package geo

import (
	"fmt"
	"photofield/internal/metrics"
	"unsafe"

	"github.com/dgraph-io/ristretto"
	"github.com/peterstace/simplefeatures/geom"
	"github.com/smilyorg/tinygpkg/gpkg"
)

type Cache struct {
	cache *ristretto.Cache
}

func NewCache() (*Cache, error) {
	g := &Cache{}
	c, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 100000,     // number of keys to track frequency of, 10x max expected key count
		MaxCost:     64_000_000, // maximum size/cost of cache
		BufferItems: 64,         // number of keys per Get buffer.
		Metrics:     true,
		Cost: func(value interface{}) int64 {
			return estimateGeometryMemorySize(value.(geom.Geometry))
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create geometry cache: %w", err)
	}
	metrics.AddRistretto("geometry_cache", c)
	g.cache = c
	return g, nil
}

func (g *Cache) Get(fid gpkg.FeatureId) (geom.Geometry, error) {
	v, ok := g.cache.Get(int64(fid))
	if !ok {
		return geom.Geometry{}, gpkg.ErrNotFound
	}
	return v.(geom.Geometry), nil
}

func (g *Cache) Set(fid gpkg.FeatureId, geom geom.Geometry) error {
	g.cache.Set(int64(fid), geom, 0)
	return nil
}

func estimateGeometryMemorySize(g geom.Geometry) int64 {
	switch g.Type() {
	case geom.TypeGeometryCollection:
		m := int64(0)
		c := g.MustAsGeometryCollection()
		for i := 0; i < c.NumGeometries(); i++ {
			m += estimateGeometryMemorySize(c.GeometryN(i))
		}
		return m
	case geom.TypePoint:
		return int64(unsafe.Sizeof(geom.Point{}))
	case geom.TypeLineString:
		l := g.MustAsLineString()
		c := l.Coordinates()
		return int64(c.Length()*c.CoordinatesType().Dimension()*8) + int64(unsafe.Sizeof(l))
	case geom.TypePolygon:
		p := g.MustAsPolygon()
		m := int64(0)
		m += estimateGeometryMemorySize(p.ExteriorRing().AsGeometry())
		for i := 0; i < p.NumInteriorRings(); i++ {
			m += estimateGeometryMemorySize(p.InteriorRingN(i).AsGeometry())
		}
		return m + int64(unsafe.Sizeof(p))
	case geom.TypeMultiPoint:
		mp := g.MustAsMultiPoint()
		m := int64(0)
		for i := 0; i < mp.NumPoints(); i++ {
			m += estimateGeometryMemorySize(mp.PointN(i).AsGeometry())
		}
		return m + int64(unsafe.Sizeof(mp))
	case geom.TypeMultiLineString:
		mls := g.MustAsMultiLineString()
		m := int64(0)
		for i := 0; i < mls.NumLineStrings(); i++ {
			m += estimateGeometryMemorySize(mls.LineStringN(i).AsGeometry())
		}
		return m + int64(unsafe.Sizeof(mls))
	case geom.TypeMultiPolygon:
		mp := g.MustAsMultiPolygon()
		m := int64(0)
		for i := 0; i < mp.NumPolygons(); i++ {
			m += estimateGeometryMemorySize(mp.PolygonN(i).AsGeometry())
		}
		return m + int64(unsafe.Sizeof(mp))
	default:
		panic("unsupported geometry type")
	}
}
