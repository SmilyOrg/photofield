package layout

import (
	"fmt"
	"math"
	"math/rand"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"time"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/s2"
)

// func toScene(pos []r2.Point, s []float64, photoHeight float64, scene *render.Scene) {
// 	for i, p := range pos {
// 		pi := &scene.Photos[i]
// 		pi.Sprite.Rect.W *= s[i] / photoHeight
// 		pi.Sprite.Rect.H *= s[i] / photoHeight
// 		pi.Sprite.Rect.X = p.X - 0.5*pi.Sprite.Rect.W
// 		pi.Sprite.Rect.Y = p.Y - 0.5*pi.Sprite.Rect.H
// 	}
// }

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
	scene.Loading = true

	scale := 0.001

	layoutFinished := metrics.Elapsed("layout")

	// h := layout.ViewportHeight

	maxlng := 0.
	if layout.ViewportWidth > layout.ViewportHeight {
		maxlng = layout.ViewportWidth * 0.5
	} else {
		maxlng = layout.ViewportHeight * 0.5
	}

	proj := s2.NewMercatorProjection(maxlng)

	// minll := s2.LatLngFromDegrees(49.26495060791964, 3.0542203644634105)
	// maxll := s2.LatLngFromDegrees(53.12427201241878, 10.52021851439191)
	// min := proj.FromLatLng(minll)
	// max := proj.FromLatLng(maxll)
	// bounds := render.Rect{
	// 	X: min.X,
	// 	Y: min.Y,
	// 	W: max.X - min.X,
	// 	H: max.Y - min.Y,
	// }

	photoHeight := layout.ImageHeight * scale

	index := 0
	for info := range infos {
		if !image.IsValidLatLng(info.LatLng) {
			continue
		}
		p := proj.FromLatLng(info.LatLng)
		// p.X -= bounds.X
		// p.Y -= bounds.Y
		// p.X *= layout.ViewportWidth / bounds.W
		// p.Y *= layout.ViewportHeight / bounds.H
		photo := render.Photo{
			Id:     info.Id,
			Sprite: render.Sprite{},
		}
		photo.Sprite.PlaceFitHeight(
			maxlng+p.X,
			maxlng-p.Y,
			photoHeight,
			float64(info.Width),
			float64(info.Height),
		)
		// photo.Sprite.Rect = photo.Sprite.Rect.Move(
		// 	render.Point{
		// 		X: -0.5 * photo.Sprite.Rect.W,
		// 		Y: -0.5 * photo.Sprite.Rect.H,
		// 	},
		// )
		scene.Photos = append(scene.Photos, photo)
		loadCounter.Set(index)
		index++
	}

	po := make([]r2.Point, len(scene.Photos))
	pa := make([]r2.Point, len(scene.Photos))
	s := make([]float64, len(scene.Photos))
	sv := make([]float64, len(scene.Photos))
	// pb := make([]r2.Point, len(scene.Photos))
	v := make([]r2.Point, len(scene.Photos))
	// a := make([]r2.Point, len(scene.Photos))

	rsrc := rand.NewSource(123)
	rnd := rand.New(rsrc)

	startSize := photoHeight * 0.0001
	jitter := startSize * 0.1
	for i, p := range scene.Photos {
		pa[i] = r2.Point{
			X: p.Sprite.Rect.X + jitter*(rnd.Float64()-0.5),
			Y: p.Sprite.Rect.Y + jitter*(rnd.Float64()-0.5),
		}
		po[i] = pa[i]
		v[i] = r2.Point{X: 0, Y: 0}
		s[i] = startSize
		sv[i] = 0.0002
		// a[i] = r2.Point{X: 0, Y: 0}
	}

	dt := 0.1
	// slowdown := 1000
	// slowdown := 1

	// minDist := layout.ImageHeight * scale * 1.6

	go func() {
		lasttotalv := 0.
		for n := 0; n < 1000; n++ {
			totald := 0.
			// clear(a)
			for i, p := range pa {
				// pi := &scene.Photos[i]
				for j := i + 1; j < len(pa); j++ {
					q := pa[j]
					// pj := &scene.Photos[j]
					// if pi.Sprite.Rect.Intersects(pj.Sprite.Rect) {
					d := q.Sub(p)
					// np := p.Add(v[i].Mul(dt)) + a*(dt*dt*0.5)
					// na := d.Mul(0.3)
					// nv := v[i].Add(a.Add(na).Mul(0.5 * dt))
					// p[i] = np
					// v[i] = nv

					// x(t+dt) = x(t) + v(t) * dt + 0.5 *dt*dt * a(t)
					// v(t+dt) = v(t) + 0.5 * dt * (a(t) + a(t+dt))

					minDist := (s[i]*0.5 + s[j]*0.5) * 1.3

					if math.Abs(d.X) > minDist || math.Abs(d.Y) > minDist {
						continue
					}

					dist := d.Norm()
					totald += dist

					if dist > minDist {
						continue
					}

					// am := math.Max(minDist-dist, 0)
					// fmt.Printf("dn: %f am: %f\n", dn, am)

					ddist := (minDist - dist) / minDist
					// a := d.Mul(ddist * 10 * dt)
					a := d.Mul(ddist * 60 * dt)
					// a := d.Mul(0 * dt)
					// ddist := (minDist - dist) / minDist
					// a := d.Normalize().Mul(0.001 / (0.01 + dist*dist) * dt)

					// pb[i] = p.Add(v[i].Mul(dt)).Add(a.Mul(0.5 * dt * dt))

					v[i] = v[i].Sub(a)
					v[j] = v[j].Add(a)
					sv[i] *= 0.3
					sv[j] *= 0.3

					// pb[i] = p.Add(v[i].Mul(dt)).Add(a.Mul(0.5 * dt * dt))

					// dn := d.Normalize()
					// a := dn.Mul(0.1 * dt * dt)
					// v[i] = v[i].Add(a.Mul(dt))
					// x_1 = x_0 + v_0 * dt + 0.5 * a * dt*dt
					// x_n1 = 2 * x_n - x_n-1 + a * dt*dt
					// x(t+dt) = x(t) + v(t) * dt + 0.5 * a(t) * dt*dt
					// a(t+dt) = ...
					// v(t+dt) = v(t) + 0.5 * (a(t) + a(t+dt)) * dt
				}
				// }
			}
			totaldorig := 0.
			totalds := 0.
			totalv := 0.
			for i := range pa {
				// pi := &scene.Photos[i]
				// pp := r2.Point{X: pi.Sprite.Rect.X, Y: pi.Sprite.Rect.Y}
				// dorig := pp.Sub(pa[i])
				dorig := po[i].Sub(pa[i])
				totaldorig += dorig.Norm()
				v[i] = v[i].Add(dorig.Mul(0.2 * dt))
				v[i] = v[i].Mul(0.98)
				// s[i] *= 1 / (1 + 0.2*dorig.Norm()) * dt
				// s[i] += (photoHeight - s[i]) * 0.1 * dt
				totalds += photoHeight - s[i]
				// s[i] -= v[i].Norm() * 0.5 * dt
				pa[i] = pa[i].Add(v[i].Mul(dt))

				// maxgrow := 0.005
				// maxdist := 0.01
				// sv[i] = maxgrow * 1 / (1 + sv[i]) * (1 - math.Min(1, dorig.Norm()/maxdist)) * dt
				// sv[i] *= 0.9
				s[i] += sv[i] * dt

				totalv += v[i].Norm() + sv[i]

				sv[i] += 0.001
				sv[i] *= 1.1
				if s[i] < photoHeight*0.0001 {
					s[i] = photoHeight * 0.0001
				}

				// pa[i] = pa[i].Add(v[i])
				// v[i] = v[i].Mul(0.99 * dt)
			}
			energy := math.Abs(totalv - lasttotalv)
			lasttotalv = totalv
			// energy := (totald + totaldorig + totalds) - total
			// total = totald + totaldorig + totalds
			fmt.Printf("n: %4d totald: %f totaldorig: %f totalds: %f energy: %f\n", n, totald, totaldorig, totalds, energy)

			for i, p := range pa {
				pi := &scene.Photos[i]
				pi.Sprite.Rect.W = s[i]
				pi.Sprite.Rect.H = s[i]
				pi.Sprite.Rect.X = p.X - 0.5*pi.Sprite.Rect.W
				pi.Sprite.Rect.Y = p.Y - 0.5*pi.Sprite.Rect.H
			}

			if energy < 0.001 {
				break
			}

			// toScene(pa, s, photoHeight, scene)
			// time.Sleep(time.Duration(dt*float64(slowdown)*1_000_000) * time.Nanosecond)
			// pa, pb = pb, pa
			// copy(pb, pa)
		}
		scene.Loading = false
	}()

	// for i, p := range pa {
	// 	pi := &scene.Photos[i]
	// 	// pi.Sprite.PlaceFitHeight(
	// 	// 	p.X,
	// 	// 	p.Y,
	// 	// 	s[i],
	// 	// 	float64(pi.Sprite.Rect.W),
	// 	// 	float64(pi.Sprite.Rect.H),
	// 	// )
	// 	// pi.Sprite.Rect.W *= s[i] / photoHeight
	// 	// pi.Sprite.Rect.H *= s[i] / photoHeight
	// 	pi.Sprite.Rect.X = p.X - 0.5*pi.Sprite.Rect.W
	// 	pi.Sprite.Rect.Y = p.Y - 0.5*pi.Sprite.Rect.H
	// }

	// a := scene.Photos
	// b := make([]render.Photo, len(a))
	// copy(b, a)

	// for n := 0; n < 100; n++ {
	// 	totald := 0.
	// 	for i, p := range a {
	// 		pp := render.Point{
	// 			X: p.Sprite.Rect.X,
	// 			Y: p.Sprite.Rect.Y,
	// 		}
	// 		for j := i + 1; j < len(a); j++ {
	// 			q := a[j]
	// 			if p.Sprite.Rect.Intersects(q.Sprite.Rect) {
	// 				qq := render.Point{
	// 					X: q.Sprite.Rect.X,
	// 					Y: q.Sprite.Rect.Y,
	// 				}
	// 				// dist := pp.Distance(qq)
	// 				dx := qq.X - pp.X
	// 				dy := qq.Y - pp.Y
	// 				totald += dx*dx + dy*dy
	// 				d := 0.5 * 0.6
	// 				p.Sprite.Rect.X = pp.X - dx*d
	// 				p.Sprite.Rect.Y = pp.Y - dy*d
	// 				q.Sprite.Rect.X = qq.X + dx*d
	// 				q.Sprite.Rect.Y = qq.Y + dy*d
	// 				b[i] = p
	// 				b[j] = q
	// 			}
	// 		}
	// 	}
	// 	fmt.Printf("n: %4d totald: %f\n", n, totald)
	// 	a, b = b, a
	// 	copy(b, a)
	// }
	// scene.Photos = a

	layoutFinished()

}
