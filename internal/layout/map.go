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
	"time"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/s2"
	"golang.org/x/exp/slices"
)

func LayoutMap(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	loadCounter := metrics.Counter{
		Name:     "load infos",
		Interval: 1 * time.Second,
	}
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

	fmt.Printf("%f\n", earthEquatorMeters/(maxExtent*2))

	index := 0
	pp := make([]r2.Point, 0)
	pi := make([]int, 0)
	placeRects := make(map[string]s2.Rect)
	for info := range infos {
		if !image.IsValidLatLng(info.LatLng) {
			continue
		}
		if !image.IsNaNLatLng(info.LatLng) {
			l, err := source.Geo.ReverseGeocode(context.TODO(), info.LatLng)
			if err != nil {
				l = ""
			}
			if l != "" {
				placeRect, ok := placeRects[l]
				if !ok {
					placeRect = s2.RectFromLatLng(info.LatLng)
					placeRects[l] = placeRect
				} else {
					placeRects[l] = placeRect.AddPoint(info.LatLng)
				}
			}
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
		pp = append(pp, p)
		pi = append(pi, index)

		scene.Photos = append(scene.Photos, photo)
		loadCounter.Set(index)
		index++
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

	vSumLast := 0.
	for n := 0; n < 1000; n++ {
		intersections := 0
		start := time.Now()
		// Collision detection incl. sweep and prune skips
		for i, p := range pp {
			hs := s[i] * 0.5
			for j := i + 1; j < len(pp); j++ {
				q := pp[j]
				d := q.Sub(p)

				// Early-out due to presorted points
				if d.X > maxExtent {
					break
				}

				if math.Abs(d.Y) > maxExtent {
					continue
				}

				minDist := (hs + s[j]*0.5) * 1.3
				if d.X > minDist || math.Abs(d.Y) > minDist {
					continue
				}

				distsq := d.X*d.X + d.Y*d.Y
				minDistSq := minDist * minDist
				if distsq > minDistSq {
					continue
				}

				intersections++

				ddist := (minDistSq - distsq) / minDistSq
				a := d.Mul(ddist * 40 * dt)

				v[i] = v[i].Sub(a)
				v[j] = v[j].Add(a)
				sv[i] *= 0.3
				sv[j] *= 0.3
			}
		}
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
		energy := math.Abs(vSum - vSumLast)
		vSumLast = vSum
		log.Printf(
			"layout map %4d with %4d intrs %4.0f m avg %4.0f m max disp %3.0f km/s %6.1f energy %8d us\n",
			n, intersections, dispSum/float64(len(pp)), dispMax, vSum/1000, energy, elapsed,
		)

		if energy < 1 {
			break
		}
	}

	for i, p := range pp {
		photo := &scene.Photos[pi[i]]
		targetSquare := render.Rect{}
		targetSquare.W = s[i] * scale
		targetSquare.H = s[i] * scale
		targetSquare.X = (maxlng+p.X)*scale - 0.5*targetSquare.W
		targetSquare.Y = (maxlng-p.Y)*scale - 0.5*targetSquare.H
		photo.Sprite.Rect = photo.Sprite.Rect.FitInside(targetSquare)
	}

	scene.Loading = false
	layoutFinished()
}
