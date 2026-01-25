package layout

import (
	// . "photofield/internal"

	"context"
	"math"
	"photofield/internal/image"
	"photofield/internal/layout/dag"
	"photofield/internal/metrics"
	"photofield/internal/render"
	"strings"
	"time"

	"github.com/gammazero/deque"
	"github.com/golang/geo/s2"
	"github.com/tdewolff/canvas"
)

func LayoutFlex(infos <-chan image.SourcedInfo, layout Layout, scene *render.Scene, source *image.Source) {

	layout.ImageSpacing = 0.02 * layout.ImageHeight
	layout.LineSpacing = 0.02 * layout.ImageHeight

	sceneMargin := 10.
	topMargin := 64.
	if strings.Contains(layout.Tweaks, "notopmargin") {
		topMargin = 0
	}

	scene.Bounds.W = layout.ViewportWidth

	rect := render.Rect{
		X: sceneMargin,
		Y: sceneMargin + topMargin,
		W: scene.Bounds.W - sceneMargin*2,
		H: 0,
	}

	scene.Solids = make([]render.Solid, 0)
	scene.Texts = make([]render.Text, 0)

	layoutPlaced := metrics.Elapsed("layout placing")

	idealHeight := math.Min(layout.ImageHeight, layout.ViewportHeight*0.9)
	auxHeight := math.Max(80, idealHeight)
	minAuxHeight := auxHeight * 0.8
	minHeight := 0.8 * idealHeight
	maxHeight := 1.2 * idealHeight

	scene.Photos = scene.Photos[:0]
	photos := make([]dag.Photo, 0)

	layoutCounter := metrics.Counter{
		Name:     "layout",
		Interval: 1 * time.Second,
	}

	auxs := make([]dag.Aux, 0)

	// Fetch all photos
	var prevLoc s2.LatLng
	var prevLocTime time.Time
	var prevLocation string
	var prevAuxTime time.Time
	nogeo := strings.Contains(layout.Tweaks, "nogeo")
	for info := range infos {
		// Skip date/location headers when shuffle sort is active (dates are meaningless)
		if !nogeo && source.Geo.Available() && !IsShuffleOrder(layout.Order) {
			photoTime := info.DateTime
			lastLocCheck := prevLocTime.Sub(photoTime)
			if lastLocCheck < 0 {
				lastLocCheck = -lastLocCheck
			}
			queryLocation := prevLocTime.IsZero() || lastLocCheck > 15*time.Minute
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
						aux := dag.Aux{
							Text: text,
						}
						auxs = append(auxs, aux)
						photos = append(photos, dag.Photo{
							Id:          image.ImageId(len(auxs) - 1),
							AspectRatio: 0.2 + float32(longestLine(text))/10,
							Aux:         true,
						})
					}
					prevLoc = info.LatLng
				}
			}
		}
		photo := dag.Photo{
			Id:          info.Id,
			AspectRatio: float32(info.AspectRatio()),
		}
		photos = append(photos, photo)
		layoutCounter.Set(len(photos))
	}

	// Create a directed acyclic graph to find the optimal layout
	root := dag.Node{
		Cost:           0,
		TotalAspect:    0,
		ShortestParent: -2,
	}

	q := deque.New[dag.Index](len(photos) / 4)
	q.PushBack(-1)
	indexToNode := make(map[dag.Index]dag.Node, len(photos))
	indexToNode[-1] = root

	maxLineWidth := rect.W

	for q.Len() > 0 {
		nodeIndex := q.PopFront()
		node := indexToNode[nodeIndex]
		totalAspect := 0.
		fallback := false
		hasAux := false

		for i := nodeIndex + 1; i < len(photos); i++ {
			photo := photos[i]
			totalAspect += float64(photo.AspectRatio)
			totalSpacing := layout.ImageSpacing * float64(i-1-nodeIndex)
			photoHeight := (maxLineWidth - totalSpacing) / totalAspect
			valid := photoHeight >= minHeight && photoHeight <= maxHeight || i == len(photos)-1 || fallback

			badness := math.Abs(photoHeight - idealHeight)
			cost := badness*badness + 10
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
			if valid {
				totalCost := node.Cost + float32(cost)
				n, ok := indexToNode[i]
				if !ok || (ok && n.Cost > totalCost) {
					n.Cost = totalCost
					n.TotalAspect = float32(totalAspect)
					n.ShortestParent = nodeIndex
				}
				if !ok && i < len(photos)-1 {
					q.PushBack(i)
				}
				indexToNode[i] = n
			}
			if photoHeight < minHeight {
				// Handle edge case where there is no other option
				// but to accept a photo that would otherwise break outside of the desired size
				if !fallback && i != len(photos)-1 && q.Len() == 0 {
					fallback = true
					for j := 0; j < 2 && i > nodeIndex; j++ {
						totalAspect -= float64(photos[i].AspectRatio)
						i--
					}
					continue
				}
				break
			}
		}
	}

	// Trace back the shortest path
	shortestPath := make([]int, 0)
	for nodeIndex := len(photos) - 1; nodeIndex != -2; {
		shortestPath = append(shortestPath, nodeIndex)
		nodeIndex = indexToNode[nodeIndex].ShortestParent
	}

	// Finally, place the photos based on the shortest path breaks
	x := 0.
	y := 0.
	idx := 0
	for i := len(shortestPath) - 2; i >= 0; i-- {
		nodeIdx := shortestPath[i]
		prevIdx := shortestPath[i+1]
		node := indexToNode[nodeIdx]
		totalSpacing := layout.ImageSpacing * float64(nodeIdx-1-prevIdx)
		imageHeight := (maxLineWidth - totalSpacing) / float64(node.TotalAspect)
		for ; idx <= nodeIdx; idx++ {
			photo := photos[idx]
			imageWidth := imageHeight * float64(photo.AspectRatio)
			if photo.Aux {
				aux := auxs[photo.Id]
				size := imageHeight * 0.5
				font := scene.Fonts.Main.Face(size, canvas.Dimgray, canvas.FontRegular, canvas.FontNormal)
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
		}
		x = 0
		y += imageHeight + layout.LineSpacing
	}

	rect.H = rect.Y + y + sceneMargin - layout.LineSpacing
	scene.Bounds.H = rect.H
	layoutPlaced()

	scene.RegionSource = PhotoRegionSource{
		Source: source,
	}
}
