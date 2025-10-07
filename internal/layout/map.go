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

	// Clustering configuration
	clusteringEnabled = true
	minClusterSize    = 20 // Don't bother for tiny datasets
)

// Grid cell identifier
type cellID struct {
	x, y int
}

// A cluster of points that can be processed independently
type Cluster struct {
	indices []int // Global indices into pp, v, s, sv arrays
}

func LayoutMap(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	scene.Bounds = render.Rect{
		X: 0,
		Y: 0,
		W: layout.ViewportWidth,
		H: layout.ViewportHeight,
	}

	// layoutFinished := metrics.Elapsed("layout")

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
		scene.FileCount = index
		scene.LoadCount = index
		scene.LoadUnit = "files"
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

	// Find clusters once based on initial positions
	// Since maxExtent limits movement, clusters remain stable across iterations
	var clusters []Cluster
	useClustering := clusteringEnabled && len(pp) > minClusterSize
	if useClustering {
		cellSize := maxExtent * 2
		grid := assignToGrid(pp, cellSize)
		clusters = mergeAdjacentCells(grid)
	}

	vSumLast := 0.
	for n := 0; n < 10; n++ {
		start := time.Now()

		var intersections, clusterCount int

		// Process collisions using pre-computed clusters
		if useClustering {
			intersections = collideClusters(clusters, pp, v, s, sv, maxExtent, dt)
			clusterCount = len(clusters)
		} else {
			// Fallback to original algorithm for small datasets
			intersections = collide(pp, v, s, sv, 0, len(pp), maxExtent, dt)
			clusterCount = 1
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
			if ndist > maxMove {
				np = po[i].Add(npd.Mul(maxMove / ndist))
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
			"layout map %4d with %4d clusters %4d intrs %4.0f m avg %4.0f m max disp %3.0f km/s %6.1f energy %8d us\n",
			n, clusterCount, intersections, dispSum/float64(len(pp)), dispMax, vSum/1000, energy, elapsed,
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
	// layoutFinished()
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

// mergeAdjacentCells groups grid cells into clusters.
// Adjacent cells (including diagonals) are merged since points in them could interact.
func mergeAdjacentCells(grid map[cellID][]int) []Cluster {
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

	// Convert to cluster slice
	clusters := make([]Cluster, 0, len(clusterMap))
	for _, indices := range clusterMap {
		clusters = append(clusters, Cluster{indices: indices})
	}

	return clusters
}

// collideClusters processes collision detection for each cluster in parallel.
func collideClusters(clusters []Cluster, pp, v []r2.Point, s, sv []float64, maxExtent, dt float64) int {
	if len(clusters) == 0 {
		return 0
	}

	// Single cluster - no need for parallelization overhead
	if len(clusters) == 1 {
		return collideCluster(clusters[0], pp, v, s, sv, maxExtent, dt)
	}

	// Multiple clusters - process in parallel
	totalInters := make([]int, len(clusters))
	var wg sync.WaitGroup

	for i := range clusters {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			totalInters[idx] = collideCluster(clusters[idx], pp, v, s, sv, maxExtent, dt)
		}(i)
	}

	wg.Wait()

	// Sum up intersections
	total := 0
	for _, count := range totalInters {
		total += count
	}
	return total
}

// collideCluster performs collision detection within a single cluster.
func collideCluster(cluster Cluster, pp, v []r2.Point, s, sv []float64, maxExtent, dt float64) int {
	inters := 0
	indices := cluster.indices

	// Check all pairs within this cluster
	for i := 0; i < len(indices); i++ {
		idx1 := indices[i]
		p := pp[idx1]
		hs := s[idx1] * 0.5

		for j := i + 1; j < len(indices); j++ {
			idx2 := indices[j]
			q := pp[idx2]
			d := q.Sub(p)

			// Distance check
			minDist := (hs + s[idx2]*0.5) * 1.3
			distsq := d.X*d.X + d.Y*d.Y
			minDistSq := minDist * minDist

			if distsq > minDistSq {
				continue
			}

			inters++

			// Apply collision forces
			ddist := (minDistSq - distsq) / minDistSq
			a := d.Mul(ddist * 40 * dt)

			v[idx1] = v[idx1].Sub(a)
			v[idx2] = v[idx2].Add(a)
			sv[idx1] *= 0.3
			sv[idx2] *= 0.3
		}
	}

	return inters
}
