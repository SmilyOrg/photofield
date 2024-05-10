package layout

import (
	// . "photofield/internal"

	"context"
	"math"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"time"

	"github.com/gammazero/deque"
	"github.com/golang/geo/s2"
	"github.com/tdewolff/canvas"
)

type FlexPhoto struct {
	Id          image.ImageId
	AspectRatio float32
	Aux         bool
}

type FlexAux struct {
	Text string
}

type FlexNode struct {
	Index       int
	Cost        float32
	TotalAspect float32
	Shortest    *FlexNode
}

func (n *FlexNode) Dot() string {
	// dot := ""

	stack := []*FlexNode{n}
	visited := make(map[int]bool)
	dot := ""
	for len(stack) > 0 {
		node := stack[0]
		stack = stack[1:]
		if visited[node.Index] {
			continue
		}
		visited[node.Index] = true
		// dot += fmt.Sprintf("%d [label=\"%d\\nCost: %.0f\\nHeight: %.0f\\nTotalAspect: %.2f\"];\n", node.Index, node.Index, node.Cost, node.ImageHeight, node.TotalAspect)
		// for _, link := range node.Links {
		// 	attr := ""
		// 	if link.Shortest == node {
		// 		attr = " [penwidth=3]"
		// 	}
		// 	dot += fmt.Sprintf("\t%d -> %d%s;\n", node.Index, link.Index, attr)
		// 	stack = append(stack, link)
		// }
	}
	return dot
}

func LayoutFlex(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.

	scene.Bounds.W = layout.ViewportWidth

	rect := render.Rect{
		X: sceneMargin,
		Y: sceneMargin + 64,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	layoutPlaced := metrics.Elapsed("layout placing")

	// row := make([]SectionPhoto, 0)

	// x := 0.
	// y := 0.

	idealHeight := layout.ImageHeight
	minHeight := 0.8 * idealHeight
	maxHeight := 1.2 * idealHeight

	// baseWidth := layout.ViewportWidth * 0.29

	scene.Photos = scene.Photos[:0]
	photos := make([]FlexPhoto, 0)

	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	auxs := make([]FlexAux, 0)

	// Fetch all photos
	var lastLoc s2.LatLng
	var lastLocTime time.Time
	var lastLocation string
	for info := range infos {
		if source.Geo.Available() {
			photoTime := info.DateTime
			lastLocCheck := lastLocTime.Sub(photoTime)
			if lastLocCheck < 0 {
				lastLocCheck = -lastLocCheck
			}
			queryLocation := lastLocTime.IsZero() || lastLocCheck > 15*time.Minute
			// fmt.Printf("lastLocTime %v photoTime %v lastLocCheck %v queryLocation %v\n", lastLocTime, photoTime, lastLocCheck, queryLocation)
			if queryLocation && image.IsValidLatLng(info.LatLng) {
				lastLocTime = photoTime
				dist := image.AngleToKm(lastLoc.Distance(info.LatLng))
				if dist > 1 {
					location, err := source.Geo.ReverseGeocode(context.TODO(), info.LatLng)
					if err == nil && location != lastLocation {
						lastLocation = location
						aux := FlexAux{
							Text: location,
						}
						auxs = append(auxs, aux)
						photos = append(photos, FlexPhoto{
							Id:          image.ImageId(len(auxs) - 1),
							AspectRatio: float32(len(location)) / 5,
							Aux:         true,
						})
					}
					lastLoc = info.LatLng
				}
			}
		}
		photo := FlexPhoto{
			Id:          info.Id,
			AspectRatio: float32(info.Width) / float32(info.Height),
		}
		photos = append(photos, photo)
		layoutCounter.Set(len(photos))
	}

	root := &FlexNode{
		Index:       -1,
		Cost:        0,
		TotalAspect: 0,
	}

	q := deque.New[*FlexNode](len(photos) / 4)
	q.PushBack(root)
	indexToNode := make(map[int]*FlexNode, len(photos))

	maxLineWidth := rect.W

	for q.Len() > 0 {
		node := q.PopFront()
		totalAspect := 0.
		fallback := false

		// fmt.Printf("queue %d\n", node.Index)
		for i := node.Index + 1; i < len(photos); i++ {
			photo := photos[i]
			totalAspect += float64(photo.AspectRatio)
			totalSpacing := layout.ImageSpacing * float64(i-1-node.Index)
			photoHeight := (maxLineWidth - totalSpacing) / totalAspect
			valid := photoHeight >= minHeight && photoHeight <= maxHeight || i == len(photos)-1 || fallback
			badness := math.Abs(photoHeight - idealHeight)
			cost := badness*badness + 10
			if i < len(photos)-1 && photos[i+1].Aux {
				cost *= 0.1
			}

			// fmt.Printf("  photo %d aspect %f total %f width %f height %f valid %v badness %f cost %f\n", i, photo.AspectRatio, totalAspect, maxLineWidth, photoHeight, valid, badness, cost)

			// Handle edge case where there is no other option
			// but to accept a photo that would otherwise break outside of the desired size
			// if i != len(photos)-1 && q.Len() == 0 {
			// 	valid = true
			// }
			if valid {
				n, ok := indexToNode[i]
				totalCost := node.Cost + float32(cost)
				if ok {
					if n.Cost > totalCost {
						n.Cost = totalCost
						n.TotalAspect = float32(totalAspect)
						n.Shortest = node
						// fmt.Printf("  node %d exists, lower cost %f\n", i, n.Cost)
					}
					// fmt.Printf("  node %d exists, keep cost %f\n", i, n.Cost)
					// }
				} else {
					n = &FlexNode{
						Index:       i,
						Cost:        totalCost,
						TotalAspect: float32(totalAspect),
						Shortest:    node,
					}
					indexToNode[i] = n
					if i < len(photos)-1 {
						q.PushBack(n)
					}
					// fmt.Printf("  node %d added with cost %f\n", i, n.Cost)
				}
				// fmt.Printf("  node %d %v cost %f\n", i, ok, n.Cost)
			}
			if photoHeight < minHeight {
				// Handle edge case where there is no other option
				// but to accept a photo that would otherwise break outside of the desired size
				if !fallback && i != len(photos)-1 && q.Len() == 0 {
					fallback = true
					for j := 0; j < 2 && i > node.Index; j++ {
						// fmt.Printf("  fallback %d\n", i)
						totalAspect -= float64(photos[i].AspectRatio)
						i--
					}
					continue
				}
				break
			}
		}
	}

	// dot := "digraph NodeGraph {\n"
	// dot += root.Dot()
	// dot += "}"
	// fmt.Println(dot)

	// Trace back the shortest path
	shortestPath := make([]*FlexNode, 0)
	for node := indexToNode[len(photos)-1]; node != nil; {
		// fmt.Printf("node %d cost %f\n", node.Index, node.Cost)
		shortestPath = append(shortestPath, node)
		node = node.Shortest
	}

	// Finally, place the photos based on the shortest path breaks
	x := 0.
	y := 0.
	idx := 0
	for i := len(shortestPath) - 2; i >= 0; i-- {
		node := shortestPath[i]
		prev := shortestPath[i+1]
		totalSpacing := layout.ImageSpacing * float64(node.Index-1-prev.Index)
		imageHeight := (maxLineWidth - totalSpacing) / float64(node.TotalAspect)
		// fmt.Printf("node %d (%d) cost %f total aspect %f height %f\n", node.Index, prev.Index, node.Cost, node.TotalAspect, imageHeight)
		for ; idx <= node.Index; idx++ {
			photo := photos[idx]
			imageWidth := imageHeight * float64(photo.AspectRatio)
			if photo.Aux {
				aux := auxs[photo.Id]
				font := scene.Fonts.Main.Face(imageHeight*0.6, canvas.Dimgray, canvas.FontRegular, canvas.FontNormal)
				text := render.NewTextFromRect(
					render.Rect{
						X: rect.X + x + imageWidth*0.01,
						Y: rect.Y + y - imageHeight*0.1,
						W: imageWidth,
						H: imageHeight,
					},
					&font,
					aux.Text,
				)
				scene.Texts = append(scene.Texts, text)
			} else {
				scene.Photos = append(scene.Photos, render.Photo{
					Id: photo.Id,
					Sprite: render.Sprite{
						Rect: render.Rect{
							X: rect.X + x,
							Y: rect.Y + y,
							W: imageWidth,
							H: imageHeight,
						},
					},
				})
			}
			x += imageWidth + layout.ImageSpacing
			// fmt.Printf("photo %d x %4.0f y %4.0f aspect %f height %f\n", idx, rect.X+x, rect.Y+y, photo.AspectRatio, imageHeight)
		}
		x = 0
		y += imageHeight + layout.LineSpacing
	}

	// fmt.Printf("photos %d indextonode %d stack %d\n", len(photos), len(indexToNode), q.Len())
	// fmt.Printf("photos %d stack %d\n", cap(photos), q.Cap())

	rect.H = rect.Y + y + sceneMargin - layout.LineSpacing
	scene.Bounds.H = rect.H
	layoutPlaced()

	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}
}
