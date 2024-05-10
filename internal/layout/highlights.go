package layout

import (
	// . "photofield/internal"

	"context"
	"log"
	"math"
	"photofield/internal/clip"
	"photofield/internal/image"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"strings"

	"time"

	"github.com/gammazero/deque"
	"github.com/golang/geo/s2"
	"github.com/tdewolff/canvas"
)

type HighlightPhoto struct {
	FlexPhoto
	Height float32
}

func longestLine(s string) int {
	lines := strings.Split(s, "\r")
	longest := 0
	for _, line := range lines {
		if len(line) > longest {
			longest = len(line)
		}
	}
	return longest
}

func LayoutHighlights(infos <-chan image.InfoEmb, layout Layout, scene *render.Scene, source *image.Source) {

	layout.ImageSpacing = math.Min(2, 0.02*layout.ImageHeight)
	layout.LineSpacing = layout.ImageSpacing

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

	idealHeight := math.Min(layout.ImageHeight, layout.ViewportHeight*0.9)
	minHeightFrac := 0.05
	auxHeight := math.Max(80, idealHeight)
	minAuxHeight := auxHeight * 0.8
	simMin := 0.6
	// simPow := 3.3
	// simPow := 0.7
	// simPow := 0.3
	// simPow := 1.8
	simPow := 1.5
	// simPow := 0.5

	// baseWidth := layout.ViewportWidth * 0.29

	// Gather all photos
	scene.Photos = scene.Photos[:0]
	photos := make([]HighlightPhoto, 0)
	var prevEmb []float32
	var prevInvNorm float32
	var prevLoc s2.LatLng
	var prevLocTime time.Time
	var prevLocation string
	var prevAuxTime time.Time
	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}
	auxs := make([]FlexAux, 0)
	for info := range infos {

		if source.Geo.Available() {
			photoTime := info.DateTime
			lastLocCheck := prevLocTime.Sub(photoTime)
			if lastLocCheck < 0 {
				lastLocCheck = -lastLocCheck
			}
			queryLocation := prevLocTime.IsZero() || lastLocCheck > 15*time.Minute
			// fmt.Printf("lastLocTime %v photoTime %v lastLocCheck %v queryLocation %v\n", lastLocTime, photoTime, lastLocCheck, queryLocation)
			if queryLocation && image.IsValidLatLng(info.LatLng) {
				prevLocTime = photoTime
				dist := image.AngleToKm(prevLoc.Distance(info.LatLng))
				if dist > 1 {
					location, err := source.Geo.ReverseGeocode(context.TODO(), info.LatLng)
					if err == nil && location != prevLocation {
						prevLocation = location
						text := ""
						if prevAuxTime.Year() != photoTime.Year() {
							text += photoTime.Format("2006\r")
						}
						if prevAuxTime.YearDay() != photoTime.YearDay() {
							text += photoTime.Format("Jan 2\rMonday\r")
						}
						prevAuxTime = photoTime
						text += location
						aux := FlexAux{
							Text: text,
						}
						auxs = append(auxs, aux)
						photos = append(photos, HighlightPhoto{
							FlexPhoto: FlexPhoto{
								Id:          image.ImageId(len(auxs) - 1),
								AspectRatio: 0.2 + float32(longestLine(text))/10,
								Aux:         true,
							},
							Height: float32(auxHeight),
						})
					}
					prevLoc = info.LatLng
				}
			}
		}

		similarity := float32(0.)
		emb := info.Embedding.Float32()
		invnorm := info.Embedding.InvNormFloat32()
		simHeight := idealHeight
		if prevEmb != nil {
			dot, err := clip.DotProductFloat32Float32(
				prevEmb,
				emb,
			)
			if err != nil {
				log.Printf("dot product error: %v", err)
			}
			similarity = dot * prevInvNorm * invnorm
			simHeight = idealHeight * math.Min(1, minHeightFrac+math.Pow(1-(float64(similarity)-simMin)/(1-simMin), simPow)*(1-minHeightFrac))
		}
		prevEmb = emb
		prevInvNorm = invnorm

		photo := HighlightPhoto{
			FlexPhoto: FlexPhoto{
				Id:          info.Id,
				AspectRatio: float32(info.Width) / float32(info.Height),
			},
			Height: float32(simHeight),
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
		hasAux := false

		// fmt.Printf("queue %d\n", node.Index)

		prevHeight := photos[0].Height

		for i := node.Index + 1; i < len(photos); i++ {
			photo := photos[i]
			totalAspect += float64(photo.AspectRatio)
			totalSpacing := layout.ImageSpacing * float64(i-1-node.Index)
			photoHeight := (maxLineWidth - totalSpacing) / totalAspect
			minHeight := 0.3 * float64(photo.Height)
			maxHeight := 1.7 * float64(photo.Height)
			valid := photoHeight >= minHeight && photoHeight <= maxHeight || i == len(photos)-1 || fallback
			// badness := math.Abs(photoHeight - idealHeight)
			badness := math.Abs(photoHeight - float64(photo.Height))
			prevDiff := 0.1 * math.Abs(float64(prevHeight-photo.Height))
			prevHeight = photo.Height
			// viewportDiff := 1000. * float64(photoHeight)
			viewportDiff := 1000. * math.Max(0, float64(photoHeight)-layout.ViewportHeight)
			cost := badness*badness + prevDiff*prevDiff + viewportDiff*viewportDiff + 10
			// Incentivise aux items to be placed at the beginning
			if i < len(photos)-1 && photos[i+1].Aux {
				cost -= 1000000
			}
			if hasAux && photoHeight < minAuxHeight {
				auxDiff := (minAuxHeight - photoHeight) * 4
				cost += auxDiff * auxDiff
			}
			if photo.Aux {
				hasAux = true
			}
			// fmt.Printf("  photo %d aspect %f total %f width %f height %f valid %v badness %f cost %f\n", i, photo.AspectRatio, totalAspect, maxLineWidth, photoHeight, valid, badness, cost)
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
					for j := 0; j < 2 && i-j > node.Index; j++ {
						totalAspect -= float64(photos[i-j].AspectRatio)
					}
					i = i - 2
					if i < node.Index+1 {
						i = node.Index + 1
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
				size := imageHeight * 0.5
				// lines := strings.Count(aux.Text, "\r") + 1
				font := scene.Fonts.Main.Face(size, canvas.Dimgray, canvas.FontRegular, canvas.FontNormal)
				// lineOffset := float64(lines-1) * size * 0.4
				padding := 2.
				text := render.Text{
					Sprite: render.Sprite{
						Rect: render.Rect{
							X: rect.X + x + padding,
							Y: rect.Y + y + padding,
							W: imageWidth - 2*padding,
							H: imageHeight - 2*padding,
						},
					},
					Font:   &font,
					Text:   aux.Text,
					HAlign: canvas.Left,
					VAlign: canvas.Bottom,
				}
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
			// fmt.Printf("photo %d aspect %f height %f\n", idx, photo.AspectRatio, photo.Height)
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

func LayoutHighlightsBasic(infos <-chan image.InfoEmb, layout Layout, scene *render.Scene, source *image.Source) {

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

	layoutPlaced := metrics.Elapsed("layout placing")
	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	row := make([]SectionPhoto, 0)

	x := 0.
	y := 0.

	simMin := 0.5
	simPow := 3.3
	// simPow := 0.7
	maxWidthFrac := 0.49
	minWidthFrac := 0.05
	baseWidth := layout.ViewportWidth * maxWidthFrac

	var prevEmb []float32
	var prevInvNorm float32

	scene.Photos = scene.Photos[:0]
	index := 0
	for info := range infos {
		photo := SectionPhoto{
			Photo: render.Photo{
				Id:     info.Id,
				Sprite: render.Sprite{},
			},
			Size: image.Size{
				X: info.Width,
				Y: info.Height,
			},
		}
		// section.infos = append(section.infos, info.SourcedInfo)

		similarity := float32(0.)
		emb := info.Embedding.Float32()
		invnorm := info.Embedding.InvNormFloat32()
		if prevEmb != nil {
			dot, err := clip.DotProductFloat32Float32(
				prevEmb,
				emb,
			)
			if err != nil {
				log.Printf("dot product error: %v", err)
			}
			similarity = dot * prevInvNorm * invnorm
		}
		prevEmb = emb
		prevInvNorm = invnorm

		// simWidth := baseWidth * math.Pow(math.Min(1., 1-(float64(similarity)-simMin)), simPow)
		simWidth := baseWidth * math.Min(1, minWidthFrac+math.Pow(1-(float64(similarity)-simMin)/(1-simMin), simPow)*(1-minWidthFrac))

		// fmt.Printf("id: %6d, similarity: %f, width: %f / %f\n", info.Id, similarity, simWidth, baseWidth)

		// aspectRatio := float64(photo.Size.X) / float64(photo.Size.Y)
		imageWidth := simWidth
		// imageHeight := imageWidth / aspectRatio

		if x+imageWidth > rect.W {

			x = 0
			for i := range row {
				photo := &row[i]
				photo.Photo.Sprite.PlaceFitHeight(
					rect.X+x,
					rect.Y+y,
					layout.ImageHeight,
					float64(photo.Size.X),
					float64(photo.Size.Y),
				)
				x += photo.Sprite.Rect.W + layout.ImageSpacing
			}
			x -= layout.ImageSpacing

			scale := layoutFitRow(row, rect, layout.ImageSpacing)

			for _, p := range row {
				scene.Photos = append(scene.Photos, p.Photo)
			}
			row = nil
			x = 0
			y += layout.ImageHeight*scale + layout.LineSpacing
		}

		photo.Photo.Sprite.PlaceFitWidth(
			rect.X+x,
			rect.Y+y,
			imageWidth,
			float64(photo.Size.X),
			float64(photo.Size.Y),
		)

		row = append(row, photo)

		x += imageWidth + layout.ImageSpacing

		layoutCounter.Set(index)
		index++
		scene.FileCount = index
	}
	for _, p := range row {
		scene.Photos = append(scene.Photos, p.Photo)
	}
	x = 0
	y += layout.ImageHeight + layout.LineSpacing

	rect.Y = y

	// newBounds := addSectionToScene(&section, scene, rect, layout, source)
	layoutPlaced()

	scene.Bounds.H = rect.Y + sceneMargin
	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}
}
