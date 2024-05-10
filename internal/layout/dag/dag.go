package dag

import (
	"photofield/internal/image"
)

type Id = image.ImageId

type Photo struct {
	Id          Id
	AspectRatio float32
	Aux         bool
}

type Aux struct {
	Text string
}

type Index int

type Node struct {
	Index          Index
	ShortestParent Index
	Cost           float32
	TotalAspect    float32
}

// type Graph[T Item] struct {
// 	items []Item
// 	q     *deque.Deque[Index]
// 	nodes map[Index]Node
// }

// func New[T Item](items []Item) *Graph[T] {
// 	g := &Graph[T]{
// 		items: items,
// 		q:     deque.New[Index](len(items) / 4),
// 		nodes: make(map[Index]Node, len(items)),
// 	}
// 	g.nodes[-1] = Node{
// 		ItemIndex:   -1,
// 		Cost:        0,
// 		TotalAspect: 0,
// 	}
// 	g.q.PushBack(0)
// 	return g
// }

// func (g *Graph[T]) Next() Node {
// 	idx := g.q.PopFront()
// 	return g.nodes[idx]
// }

// func (g *Graph[T]) Add(i Index, cost float32) {
// 	n, ok := g.nodes[i]
// 	if ok {
// 		if n.Cost > cost {
// 			n.Cost = cost
// 			n.ShortestParent = i
// 			// fmt.Printf("  node %d exists, lower cost %f\n", i, n.Cost)
// 		}
// 		// fmt.Printf("  node %d exists, keep cost %f\n", i, n.Cost)
// 		// }
// 	} else {
// 		n = &FlexNode{
// 			Index:       i,
// 			Cost:        totalCost,
// 			TotalAspect: float32(totalAspect),
// 			Shortest:    node,
// 		}
// 		indexToNode[i] = n
// 		if i < len(photos)-1 {
// 			q.PushBack(n)
// 		}
// 		// fmt.Printf("  node %d added with cost %f\n", i, n.Cost)
// 	}
// }
