package layout

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"runtime"
	"sync"
	"time"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/s2"
	"github.com/tdewolff/canvas"
	"golang.org/x/exp/slices"
)

type Cluster struct {
	s2.Point
	// s2.Loop
	name   string
	radius float64
	count  int
}

func LayoutMap(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	scene.Bounds = render.Rect{
		X: 0,
		Y: 0,
		W: layout.ViewportWidth,
		H: layout.ViewportHeight,
	}

	layoutFinished := metrics.Elapsed("layout")

	earthEquatorMeters := 40075017.

	maxlng := 0.

	maxlng = earthEquatorMeters * 0.5

	scale := 0.
	if layout.ViewportWidth > layout.ViewportHeight {
		scale = layout.ViewportWidth / (maxlng * 2)
	} else {
		scale = layout.ViewportHeight / (maxlng * 2)
	}

	proj := s2.NewMercatorProjection(maxlng)

	maxSize := 2000.
	maxDist := 1500.
	maxExtent := maxDist + maxSize*0.5*1.0001
	minSize := 1.
	startSize := 1.

	index := 0
	pp := make([]r2.Point, 0)
	pi := make([]int, 0)
	photos := make([]render.Photo, 0)
	// clusters := s2.NewShapeIndex()
	clusters := make([]Cluster, 0)
	// clusterMaxAngle := s1.ChordAngleFromAngle(image.KmToAngle(10))
	clusterMaxAngle := image.KmToAngle(10)
	loadCounter := metrics.Counter{
		Name:     "load infos",
		Interval: 1 * time.Second,
	}
	for info := range infos {
		if !image.IsValidLatLng(info.LatLng) {
			continue
		}
		p := proj.FromLatLng(info.LatLng)
		photo := render.Photo{
			Id: info.Id,
			Sprite: render.Sprite{
				Rect: render.Rect{
					W: float64(info.Width),
					H: float64(info.Height),
				},
			},
		}
		// fmt.Printf("point %f %f\n", info.LatLng.Lat.Degrees(), info.LatLng.Lng.Degrees())
		s2p := s2.PointFromLatLng(info.LatLng)
		addCluster := false
		if len(clusters) == 0 {
			addCluster = true
		} else {
			updated := false
			for i, c := range clusters {
				// fmt.Printf("  cluster %d %f %f\n", i, s2.LatLngFromPoint(c.Point).Lat.Degrees(), s2.LatLngFromPoint(c.Point).Lng.Degrees())
				dist := c.Point.Distance(s2p)
				if dist.Radians() < clusterMaxAngle.Radians() {
					c.count++
					frac := 1. / float64(c.count)
					c.Point = s2.Interpolate(
						frac,
						c.Point,
						s2p,
					)
					c.radius += image.AngleToKm(dist) * frac
					// fmt.Printf("    update cluster %f %f\n", frac, image.AngleToKm(dist))
					// fmt.Printf("    update cluster %f %f\n", s2.LatLngFromPoint(c.Point).Lat.Degrees(), s2.LatLngFromPoint(c.Point).Lng.Degrees())
					clusters[i] = c
					updated = true
					break
				}
			}

			// fmt.Printf("query %f %f\n", s2.LatLngFromPoint(s2p).Lat.Degrees(), s2.LatLngFromPoint(s2p).Lng.Degrees())
			// eq := s2.NewClosestEdgeQuery(clusters, s2.NewClosestEdgeQueryOptions().DistanceLimit(clusterMaxAngle))
			// eq := s2.NewClosestEdgeQuery(clusters, s2.NewClosestEdgeQueryOptions())
			// target := s2.NewMinDistanceToPointTarget(s2p)

			// for _, r := range eq.FindEdges(target) {
			// fmt.Printf("result %d %f\n", r.ShapeID(), image.AngleToKm(r.Distance().Angle()))
			// s := clusters.Shape(r.ShapeID())
			// fmt.Printf("result %s\n", s.String())
			// cluster := s.(*Cluster)
			// clusters.Remove(s)
			// cluster.count++
			// frac := 1. / float64(cluster.count)
			// cluster.PointVector[0] = s2.Interpolate(
			// 	frac,
			// 	cluster.PointVector[0],
			// 	s2p,
			// )
			// clusters.Add(cluster)
			// fmt.Printf("update cluster %f %f\n", s2.LatLngFromPoint(cluster.PointVector[0]).Lat.Degrees(), s2.LatLngFromPoint(cluster.PointVector[0]).Lng.Degrees())
			// fmt.Printf("update cluster %s\n", cluster.name)
			// 	updated = true
			// 	break
			// }
			// eq.Reset()
			if !updated {
				addCluster = true
			}
			// if eq.IsDistanceGreater(s2.NewMaxDistanceToPointTarget(s2p), clusterMaxAngle) {
			// 	addCluster = true
			// }
		}
		if addCluster {
			// fmt.Printf("  new cluster %f %f\n", info.LatLng.Lat.Degrees(), info.LatLng.Lng.Degrees())
			// clusters.Add(&Cluster{
			// 	// PointVector: s2.PointVector{s2p},
			// 	Loop:   *s2.RegularLoop(s2p, s1.Angle(clusterMaxAngle), 12),
			// 	name:   info.String(),
			// 	radius: 0,
			// 	count:  1,
			// })
			// clusters.Add(s2.RegularLoop(s2p, s1.Angle(clusterMaxAngle), 12))
			// clusters.Add(s2.PolylineFromLatLngs([]s2.LatLng{info.LatLng}))
			// idx := clusters.Add(&s2.PointVector{
			// 	s2.PointFromCoords(rand.Float64(), rand.Float64(), rand.Float64()),
			// })
			// idx = clusters.Add(&s2.PointVector{
			// 	s2.PointFromCoords(rand.Float64(), rand.Float64(), rand.Float64()),
			// })
			// idx = clusters.Add(&s2.PointVector{
			// 	s2.PointFromCoords(rand.Float64(), rand.Float64(), rand.Float64()),
			// })
			// idx = clusters.Add(&s2.PointVector{
			// 	s2.PointFromCoords(rand.Float64(), rand.Float64(), rand.Float64()),
			// })
			// fmt.Printf("before build %d\n", idx)
			// clusters.Build()
			clusters = append(clusters, Cluster{
				Point:  s2p,
				name:   fmt.Sprintf("cluster %d", len(clusters)),
				radius: 0,
				count:  1,
			})
			// fmt.Printf("after build\n")
		}
		pp = append(pp, p)
		pi = append(pi, len(pi))
		photos = append(photos, photo)
		loadCounter.Set(index)
		index++
		scene.FileCount = index
		scene.LoadCount = index
		scene.LoadUnit = "files"
	}

	for i, c := range clusters {

		latlng := s2.LatLngFromPoint(c.Point)

		location, err := source.Geo.ReverseGeocode(context.TODO(), latlng)
		if err == nil {
			c.name = location
		}

		fmt.Printf("cluster %d %f %f %f\n", i, s2.LatLngFromPoint(c.Point).Lat.Degrees(), s2.LatLngFromPoint(c.Point).Lng.Degrees(), c.radius)
		p := proj.FromLatLng(latlng)
		font := scene.Fonts.Main.Face(10, canvas.Dimgray, canvas.FontRegular, canvas.FontNormal)

		// size := math.Max(0.5, c.radius*0.1)

		// square := render.Rect{}
		// square.W = size
		// square.H = size
		// square.X = (maxlng+p.X)*scale - 0.5*square.W
		// square.Y = (maxlng-p.Y)*scale - 0.5*square.H

		// scene.Solids = append(scene.Solids, render.Solid{
		// 	Sprite: render.Sprite{
		// 		Rect: square,
		// 	},
		// 	Color: canvas.Lightgray,
		// })

		// square.Y -= math.Max(2, c.radius)

		size := 30.

		bg := render.Rect{}
		bg.W = size
		bg.H = size * 0.2
		bg.X = (maxlng+p.X)*scale - 0.5*bg.W
		bg.Y = (maxlng-p.Y)*scale - bg.H - math.Max(1, c.radius)

		scene.Solids = append(scene.Solids, render.Solid{
			Sprite: render.Sprite{
				Rect: bg,
			},
			Color: canvas.Lightgray,
		})

		text := render.Text{
			Text: c.name,
			Sprite: render.Sprite{
				Rect: bg,
			},
			Font:   &font,
			HAlign: canvas.Center,
			VAlign: canvas.Center,
		}
		scene.Texts = append(scene.Texts, text)
	}

	n := len(pp)
	po := make([]r2.Point, n)
	v := make([]r2.Point, n)
	s := make([]float64, n)
	sv := make([]float64, n)

	rsrc := rand.NewSource(123)
	rnd := rand.New(rsrc)

	slices.SortFunc(pi, func(i, j int) int {
		return cmp.Compare(pp[i].X, pp[j].X)
	})

	jitter := startSize * 0.1
	for i := range pi {
		j := pi[i]
		p := pp[j]
		po[i] = p
		v[i] = r2.Point{X: 0, Y: 0}
		s[i] = startSize
		sv[i] = 16
	}

	for i := range pi {
		pp[i] = po[i].Add(r2.Point{
			X: jitter * (rnd.Float64() - 0.5),
			Y: jitter * (rnd.Float64() - 0.5),
		})
	}

	dt := 0.1

	workerNum := runtime.NumCPU()
	workerBatch := len(pp) / workerNum

	vSumLast := 0.
	for n := 0; n < 1000; n++ {
		intersections := 0
		start := time.Now()
		wg := &sync.WaitGroup{}
		wg.Add(workerNum)
		for w := 0; w < workerNum; w++ {
			ia := w * workerBatch
			ib := (w + 1) * workerBatch
			if w == workerNum-1 {
				ib = len(pp)
			}
			go func() {
				intersections += collide(pp, v, s, sv, ia, ib, maxExtent, dt)
				wg.Done()
			}()
		}
		wg.Wait()
		elapsed := int(time.Since(start).Microseconds())

		dispSum := 0.
		dispMax := 0.
		vSum := 0.
		for i := range pp {

			dorig := po[i].Sub(pp[i])
			dist := dorig.Norm()
			dispSum += dist
			if dist > dispMax {
				dispMax = dist
			}
			v[i] = v[i].Add(dorig.Mul(0.1 * dt))
			v[i] = v[i].Mul(0.98)

			np := pp[i].Add(v[i].Mul(dt))
			npd := np.Sub(po[i])
			ndist := npd.Norm()
			if ndist > maxDist {
				np = po[i].Add(npd.Mul(maxDist / ndist))
				v[i] = r2.Point{}
				sv[i] = 0
			}
			pp[i] = np

			s[i] += sv[i] * dt

			vSum += v[i].Norm() + sv[i]

			sv[i] += 100 * dt
			sv[i] *= 1.01

			if s[i] > maxSize {
				s[i] = maxSize
				sv[i] = 0
			}
			if s[i] < minSize {
				s[i] = minSize
				sv[i] = 0
			}

		}
		energy := vSum - vSumLast
		if energy < 0 {
			energy = -energy
		}
		vSumLast = vSum
		log.Printf(
			"layout map %4d with %4d intrs %4.0f m avg %4.0f m max disp %3.0f km/s %6.1f energy %8d us\n",
			n, intersections, dispSum/float64(len(pp)), dispMax, vSum/1000, energy, elapsed,
		)

		scene.LoadCount = n
		scene.LoadUnit = "iterations"

		if energy < 10 {
			break
		}
	}

	for i, p := range pp {
		photo := &photos[pi[i]]
		square := render.Rect{}
		square.W = s[i] * scale
		square.H = s[i] * scale
		square.X = (maxlng+p.X)*scale - 0.5*square.W
		square.Y = (maxlng-p.Y)*scale - 0.5*square.H
		photo.Sprite.Rect = photo.Sprite.Rect.FitInside(square)
	}

	scene.Photos = photos

	scene.LoadCount = 0
	scene.LoadUnit = ""
	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}
	layoutFinished()
}

// Collision detection incl. sweep and prune skips
func collide(pp, v []r2.Point, s, sv []float64, ia, ib int, maxExtent, dt float64) int {
	inters := 0
	for i := ia; i < ib; i++ {
		p := pp[i]
		hs := s[i] * 0.5
		for j := i + 1; j < len(pp); j++ {
			q := pp[j]
			d := q.Sub(p)

			// Early-out due to presorted points
			if d.X > maxExtent {
				break
			}

			dyabs := d.Y
			if dyabs < 0 {
				dyabs = -dyabs
			}

			if dyabs > maxExtent {
				continue
			}

			minDist := (hs + s[j]*0.5) * 1.3
			if d.X > minDist || dyabs > minDist {
				continue
			}

			distsq := d.X*d.X + d.Y*d.Y
			minDistSq := minDist * minDist
			if distsq > minDistSq {
				continue
			}

			inters++

			ddist := (minDistSq - distsq) / minDistSq
			a := d.Mul(ddist * 40 * dt)

			v[i] = v[i].Sub(a)
			v[j] = v[j].Add(a)
			sv[i] *= 0.3
			sv[j] *= 0.3
		}
	}
	return inters
}
