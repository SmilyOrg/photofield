package layout

import (
	// . "photofield/internal"

	"fmt"
	"math"
	"photofield/internal/image"
	"photofield/internal/render"
	"time"
)

type FlexPhoto struct {
	Id          image.ImageId
	AspectRatio float64
}

type FlexNode struct {
	Index       int
	Cost        float64
	ImageHeight float64
	TotalAspect float64
	Links       []*FlexNode
	Shortest    *FlexNode
}

// func (n *FlexNode) Dot() string {
// 	// dot := ""
// 	dot := fmt.Sprintf("%d [label=\"%d\\nCost: %.0f\\nHeight: %.0f\"];\n", n.Index, n.Index, n.Cost, n.ImageHeight)
// 	for _, link := range n.Links {
// 		dot += fmt.Sprintf("\t%d -> %d;\n", n.Index, link.Index)
// 		// dot += fmt.Sprintf("\t%d -> %d;\n", n.Index, link.Index)
// 		dot += link.Dot()
// 	}
// 	return dot
// }

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
		dot += fmt.Sprintf("%d [label=\"%d\\nCost: %.0f\\nHeight: %.0f\\nTotalAspect: %.2f\"];\n", node.Index, node.Index, node.Cost, node.ImageHeight, node.TotalAspect)
		for _, link := range node.Links {
			attr := ""
			if link.Shortest == node {
				attr = " [penwidth=3]"
			}
			dot += fmt.Sprintf("\t%d -> %d%s;\n", node.Index, link.Index, attr)
			stack = append(stack, link)
		}
	}

	// dot := fmt.Sprintf("%d [label=\"%d\\nCost: %.0f\\nHeight: %.0f\"];\n", n.Index, n.Index, n.Cost, n.ImageHeight)
	// for _, link := range n.Links {
	// 	dot += fmt.Sprintf("\t%d -> %d;\n", n.Index, link.Index)
	// 	// dot += fmt.Sprintf("\t%d -> %d;\n", n.Index, link.Index)
	// 	dot += link.Dot()
	// }
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

	// section := Section{}

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	// layoutPlaced := metrics.Elapsed("layout placing")
	// layoutCounter := metrics.Counter{
	// 	Name:     "layout",
	// 	Interval: 1 * time.Second,
	// }

	// row := make([]SectionPhoto, 0)

	// x := 0.
	// y := 0.

	idealHeight := layout.ImageHeight
	minHeight := 0.8 * idealHeight
	maxHeight := 1.2 * idealHeight

	// baseWidth := layout.ViewportWidth * 0.29

	photos := make([]FlexPhoto, 0)

	for info := range infos {
		photo := FlexPhoto{
			Id:          info.Id,
			AspectRatio: float64(info.Width) / float64(info.Height),
		}
		photos = append(photos, photo)
	}

	for startTime := time.Now(); time.Since(startTime) < 10*time.Second; {

		scene.Photos = scene.Photos[:0]
		root := &FlexNode{
			Index:       -1,
			Cost:        0,
			Links:       nil,
			TotalAspect: 0,
		}
		stack := []*FlexNode{root}
		indexToNode := make(map[int]*FlexNode)

		maxLineWidth := rect.W

		for len(stack) > 0 {
			node := stack[0]
			stack = stack[1:]

			totalAspect := 0.

			// fmt.Printf("stack %d\n", node.Index)

			for i := node.Index + 1; i < len(photos); i++ {
				photo := photos[i]
				totalAspect += photo.AspectRatio
				totalSpacing := layout.ImageSpacing * float64(i-1-node.Index)
				photoHeight := (maxLineWidth - totalSpacing) / totalAspect
				valid := photoHeight >= minHeight && photoHeight <= maxHeight || i == len(photos)-1
				badness := math.Abs(photoHeight - idealHeight)
				cost := badness*badness + 10
				// fmt.Printf("  photo %d aspect %f total %f width %f height %f valid %v badness %f cost %f\n", i, photo.AspectRatio, totalAspect, maxLineWidth, photoHeight, valid, badness, cost)
				// Handle edge case where there is no other option
				// but to accept a photo that would otherwise break outside of the desired size
				if photoHeight < minHeight && len(stack) == 0 {
					valid = true
				}
				if valid {
					n, ok := indexToNode[i]
					totalCost := node.Cost + cost
					if ok {
						if n.Cost > totalCost {
							n.Cost = totalCost
							n.TotalAspect = totalAspect
							n.Shortest = node
							// fmt.Printf("  node %d exists, lower cost %f\n", i, n.Cost)
						} else {
							// fmt.Printf("  node %d exists, keep cost %f\n", i, n.Cost)
						}
					} else {
						n = &FlexNode{
							Index:       i,
							Cost:        totalCost,
							ImageHeight: photoHeight,
							Links:       nil,
							TotalAspect: totalAspect,
							Shortest:    node,
						}
						indexToNode[i] = n
						if i < len(photos)-1 {
							stack = append(stack, n)
						}
						// fmt.Printf("  node %d added with cost %f total aspect %f\n", i, n.Cost, n.TotalAspect)
					}
					node.Links = append(node.Links, n)
				}
				if photoHeight < minHeight {
					break
				} else if photoHeight > maxHeight {
					continue
				}
			}
		}

		// dot := "digraph NodeGraph {\n"
		// dot += root.Dot()
		// dot += "}"
		// fmt.Println(dot)

		// fmt.Printf("photos %d\n", len(photos))

		shortestPath := make([]*FlexNode, 0)
		for node := indexToNode[len(photos)-1]; node != nil; {
			// fmt.Printf("node %d cost %f\n", node.Index, node.Cost)
			shortestPath = append(shortestPath, node)
			node = node.Shortest
		}

		// fmt.Printf("max line width %f\n", maxLineWidth)
		x := 0.
		y := 0.
		idx := 0
		for i := len(shortestPath) - 2; i >= 0; i-- {
			node := shortestPath[i]
			prev := shortestPath[i+1]
			totalSpacing := layout.ImageSpacing * float64(node.Index-1-prev.Index)
			imageHeight := (maxLineWidth - totalSpacing) / node.TotalAspect
			// fmt.Printf("node %d (%d) cost %f total aspect %f height %f\n", node.Index, prev.Index, node.Cost, node.TotalAspect, imageHeight)
			for ; idx <= node.Index; idx++ {
				photo := photos[idx]
				imageWidth := imageHeight * photo.AspectRatio
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
				x += imageWidth + layout.ImageSpacing
				// fmt.Printf("photo %d aspect %f\n", idx, photo.AspectRatio)
			}
			x = 0
			y += imageHeight + layout.LineSpacing
		}

		// idx := 0
		// for node := root; node != nil; {

		// 	bestCost := math.MaxFloat64
		// 	var bestNode *FlexNode
		// 	for _, link := range node.Links {
		// 		if link.Cost < bestCost {
		// 			bestCost = link.Cost
		// 			bestNode = link
		// 		}
		// 	}
		// 	node = bestNode
		// 	if node == nil {
		// 		break
		// 	}
		// 	imageHeight := maxLineWidth / node.TotalAspect
		// 	fmt.Printf("node %d cost %f total aspect %f height %f\n", node.Index, node.Cost, node.TotalAspect, imageHeight)
		// 	for ; idx <= node.Index; idx++ {
		// 		photo := photos[idx]
		// 		imageWidth := imageHeight * photo.AspectRatio
		// 		scene.Photos = append(scene.Photos, render.Photo{
		// 			Id: photo.Id,
		// 			Sprite: render.Sprite{
		// 				Rect: render.Rect{
		// 					X: rect.X + x,
		// 					Y: rect.Y + y,
		// 					W: imageWidth,
		// 					H: imageHeight,
		// 				},
		// 			},
		// 		})
		// 		x += imageWidth
		// 		fmt.Printf("photo %d aspect %f\n", idx, photo.AspectRatio)
		// 	}
		// 	x = 0
		// 	y += imageHeight
		// }

		// index := 0
		// for info := range infos {
		// 	photo := SectionPhoto{
		// 		Photo: render.Photo{
		// 			Id:     info.Id,
		// 			Sprite: render.Sprite{},
		// 		},
		// 		Size: image.Size{
		// 			X: info.Width,
		// 			Y: info.Height,
		// 		},
		// 	}

		// 	imageWidth := baseWidth
		// 	// section.infos = append(section.infos, info.SourcedInfo)

		// 	if x+imageWidth > rect.W {
		// 		for _, p := range row {
		// 			scene.Photos = append(scene.Photos, p.Photo)
		// 		}
		// 		row = nil
		// 		x = 0
		// 		y += layout.ImageHeight + layout.LineSpacing
		// 	}

		// 	photo.Photo.Sprite.PlaceFitWidth(
		// 		rect.X+x,
		// 		rect.Y+y,
		// 		imageWidth,
		// 		float64(photo.Size.X),
		// 		float64(photo.Size.Y),
		// 	)

		// 	row = append(row, photo)

		// 	x += imageWidth + layout.ImageSpacing

		// 	layoutCounter.Set(index)
		// 	index++
		// 	scene.FileCount = index
		// }
		// for _, p := range row {
		// 	scene.Photos = append(scene.Photos, p.Photo)
		// }
		// x = 0
		// y += layout.ImageHeight + layout.LineSpacing

		// rect.Y = y

		// newBounds := addSectionToScene(&section, scene, rect, layout, source)
		// layoutPlaced()

		scene.Bounds.H = rect.Y + y + sceneMargin
	}

	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}
}
