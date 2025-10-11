package layout

import (
	"cmp"
	"log"
	"math"
	"math/rand"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"sync"
	"time"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/s2"
	"golang.org/x/exp/slices"
)

const (
	maxMove = 1500.0
	maxSize = 2000.0
)

// Grid cell identifier
type cellID struct {
	x, y int
}

// A cluster of points that can be processed independently
type Cluster struct {
	pp []r2.Point // positions
	po []r2.Point // original positions
	v  []r2.Point // velocities
	s  []float64  // sizes
	sv []float64  // size velocities
	pi []int      // original indices for mapping back to photos
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

	maxExtent := maxMove + maxSize*0.5*1.0001
	minSize := 1.
	startSize := 1.

	index := 0
	pp := make([]r2.Point, 0)
	pi := make([]int, 0)
	photos := make([]render.Photo, 0)
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
		pp = append(pp, p)
		pi = append(pi, len(pi))
		photos = append(photos, photo)
		loadCounter.Set(index)
		index++
		scene.LoadUnit = "files"
		scene.FileCount = index
		scene.LoadCount = index
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
	maxTime := 10 * time.Second
	maxIterations := 10000

	// Find clusters once based on initial positions
	// Since maxExtent limits movement, clusters remain stable across iterations
	// Always use clustering for consistent code path and better parallelization
	cellSize := maxExtent * 2
	grid := assignToGrid(pp, cellSize)
	clusters := mergeAdjacentCells(grid, pp, po, v, s, sv, pi)

	vSumLast := 0.
	layoutStart := time.Now()
	lastLogTime := layoutStart

	for n := 0; n < maxIterations; n++ {
		start := time.Now()
		layoutElapsed := start.Sub(layoutStart)
		if layoutElapsed > maxTime {
			log.Printf("layout map timeout after %v\n", layoutElapsed)
			scene.LoadUnit = "good enough 🤷‍♀️"
			time.Sleep(1 * time.Second)
			break
		}

		var intersections, clusterCount int
		var dispSum, dispMax, vSum float64

		// Process collisions and physics using clusters (always)
		clusterCount = len(clusters)

		// Process each cluster in parallel
		totalInters := make([]int, len(clusters))
		clusterResults := make([][3]float64, len(clusters)) // [dispSum, dispMax, vSum]
		var wg sync.WaitGroup

		for i := range clusters {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				cluster := &clusters[idx]

				// Collision detection using sweep and prune on sorted points
				totalInters[idx] = collideClusterSorted(cluster, maxExtent, dt)

				// Physics update for this cluster
				ds, dm, vs := processClusterPhysics(cluster, dt, maxMove, minSize, maxSize)
				clusterResults[idx] = [3]float64{ds, dm, vs}
			}(i)
		}

		wg.Wait()

		// Aggregate results (no need to merge back to global arrays yet)
		for _, count := range totalInters {
			intersections += count
		}
		for _, result := range clusterResults {
			dispSum += result[0]
			if result[1] > dispMax {
				dispMax = result[1]
			}
			vSum += result[2]
		}

		elapsed := int(time.Since(start).Microseconds())
		energy := math.Abs(vSum-vSumLast) * 1000 / float64(1+intersections)
		vSumLast = vSum

		// Log once per second
		if time.Since(lastLogTime) >= 1*time.Second {
			log.Printf(
				"layout map %4d with %4d clusters %4d intrs %4.0f m avg %4.0f m max disp %3.0f km/s %6.0f energy %8d us\n",
				n, clusterCount, intersections, dispSum/float64(len(pp)), dispMax, vSum/1000, energy, elapsed,
			)
			lastLogTime = time.Now()
		}

		scene.LoadCount = n
		scene.LoadUnit = "iterations"

		if energy < 10 {
			break
		}
	}

	// Merge results back to global arrays only once, after all iterations
	mergeFromClusters(clusters, pp, po, v, s, sv, pi)

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

// assignToGrid assigns each point to a grid cell based on position.
// Cell size is chosen so points in non-adjacent cells cannot interact.
func assignToGrid(pp []r2.Point, cellSize float64) map[cellID][]int {
	grid := make(map[cellID][]int)

	for i, p := range pp {
		cell := cellID{
			x: int(math.Floor(p.X / cellSize)),
			y: int(math.Floor(p.Y / cellSize)),
		}
		grid[cell] = append(grid[cell], i)
	}

	return grid
}

// unionFind implements Union-Find with path compression for grouping cells.
type unionFind struct {
	parent map[cellID]cellID
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[cellID]cellID),
	}
}

func (uf *unionFind) find(cell cellID) cellID {
	// Initialize if not seen
	if _, exists := uf.parent[cell]; !exists {
		uf.parent[cell] = cell
		return cell
	}

	// Path compression
	if uf.parent[cell] != cell {
		uf.parent[cell] = uf.find(uf.parent[cell])
	}
	return uf.parent[cell]
}

func (uf *unionFind) union(cell1, cell2 cellID) {
	root1 := uf.find(cell1)
	root2 := uf.find(cell2)

	if root1 != root2 {
		uf.parent[root1] = root2
	}
}

// mergeAdjacentCells groups grid cells into clusters and splits data directly.
// Adjacent cells (including diagonals) are merged since points in them could interact.
func mergeAdjacentCells(grid map[cellID][]int, pp, po, v []r2.Point, s, sv []float64, pi []int) []Cluster {
	if len(grid) == 0 {
		return nil
	}

	uf := newUnionFind()

	// For each cell, check its 8 neighbors (and itself = 9 cells total)
	for cell := range grid {
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				neighbor := cellID{x: cell.x + dx, y: cell.y + dy}

				// If neighbor exists in grid, merge with current cell
				if _, exists := grid[neighbor]; exists {
					uf.union(cell, neighbor)
				}
			}
		}
	}

	// Group cells by their root
	clusterMap := make(map[cellID][]int)
	for cell, indices := range grid {
		root := uf.find(cell)
		clusterMap[root] = append(clusterMap[root], indices...)
	}

	// Convert to cluster slice with data directly split
	clusters := make([]Cluster, 0, len(clusterMap))
	for _, indices := range clusterMap {
		n := len(indices)

		// Build list of global indices sorted by X upfront so we copy only once.
		sorted := make([]int, len(indices))
		copy(sorted, indices)
		slices.SortFunc(sorted, func(a, b int) int { return cmp.Compare(pp[a].X, pp[b].X) })

		cluster := Cluster{
			pp: make([]r2.Point, n),
			po: make([]r2.Point, n),
			v:  make([]r2.Point, n),
			s:  make([]float64, n),
			sv: make([]float64, n),
			pi: make([]int, n),
		}
		for i, globalIdx := range sorted {
			cluster.pp[i] = pp[globalIdx]
			cluster.po[i] = po[globalIdx]
			cluster.v[i] = v[globalIdx]
			cluster.s[i] = s[globalIdx]
			cluster.sv[i] = sv[globalIdx]
			cluster.pi[i] = pi[globalIdx]
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// mergeFromClusters merges the results back into the global arrays
func mergeFromClusters(clusters []Cluster, pp, po, v []r2.Point, s, sv []float64, pi []int) {
	for _, cluster := range clusters {
		for i, globalIdx := range cluster.pi {
			pp[globalIdx] = cluster.pp[i]
			po[globalIdx] = cluster.po[i]
			v[globalIdx] = cluster.v[i]
			s[globalIdx] = cluster.s[i]
			sv[globalIdx] = cluster.sv[i]
			pi[globalIdx] = cluster.pi[i]
		}
	}
}

// processClusterPhysics handles the physics update for a single cluster
func processClusterPhysics(cluster *Cluster, dt, maxMove, minSize, maxSize float64) (dispSum, dispMax, vSum float64) {
	for i := range cluster.pp {
		dorig := cluster.po[i].Sub(cluster.pp[i])
		dist := dorig.Norm()
		dispSum += dist
		if dist > dispMax {
			dispMax = dist
		}
		cluster.v[i] = cluster.v[i].Add(dorig.Mul(0.1 * dt))
		cluster.v[i] = cluster.v[i].Mul(0.98)

		np := cluster.pp[i].Add(cluster.v[i].Mul(dt))
		npd := np.Sub(cluster.po[i])
		ndist := npd.Norm()
		if ndist > maxMove {
			np = cluster.po[i].Add(npd.Mul(maxMove / ndist))
			cluster.v[i] = r2.Point{}
			cluster.sv[i] = 0
		}
		cluster.pp[i] = np

		cluster.s[i] += cluster.sv[i] * dt

		vSum += cluster.v[i].Norm() + cluster.sv[i]

		cluster.sv[i] += 100 * dt
		cluster.sv[i] *= 1.01

		if cluster.s[i] > maxSize {
			cluster.s[i] = maxSize
			cluster.sv[i] = 0
		}
		if cluster.s[i] < minSize {
			cluster.s[i] = minSize
			cluster.sv[i] = 0
		}
	}
	return dispSum, dispMax, vSum
}

// collideClusterSorted performs collision detection on a sorted cluster using sweep and prune
func collideClusterSorted(cluster *Cluster, maxExtent, dt float64) int {
	pp := cluster.pp
	v := cluster.v
	s := cluster.s
	sv := cluster.sv

	inters := 0
	for i := 0; i < len(pp); i++ {
		p := pp[i]
		hs := s[i] * 0.5
		for j := i + 1; j < len(pp); j++ {
			q := pp[j]
			d := q.Sub(p)
			if d.X > maxExtent {
				break
			}
			dyabs := math.Abs(d.Y)
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
