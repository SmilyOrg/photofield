package rangetree

import (
	"fmt"

	"github.com/petar/GoLLRB/llrb"
)

type Range struct {
	Low, High int
}

func (r Range) Less(than llrb.Item) bool {
	return r.High < than.(Range).Low
}

func (r Range) IsZero() bool {
	return r.Low == 0 && r.High == 0
}

func (r Range) String() string {
	return fmt.Sprintf("[%d,%d]", r.Low, r.High)
}

func FromTo(low, high int) Range {
	return Range{Low: low, High: high}
}

type Tree struct {
	tree *llrb.LLRB
}

func New() *Tree {
	return &Tree{
		tree: llrb.New(),
	}
}

func (t *Tree) Find(x int) Range {
	item := t.tree.Get(Range{Low: x, High: x})
	if item == nil {
		return Range{}
	}
	return item.(Range)
}

func (t *Tree) Add(r Range) {
	low := t.Find(r.Low - 1)
	high := t.Find(r.High + 1)
	if !low.IsZero() {
		t.tree.Delete(low)
		r.Low = low.Low
	}
	if !high.IsZero() {
		t.tree.Delete(high)
		r.High = high.High
	}
	t.tree.ReplaceOrInsert(r)
}

func (t *Tree) AddInt(x int) {
	t.Add(Range{Low: x, High: x})
}

func (t *Tree) InvertInt(x int) {
	if t.Contains(x) {
		t.Subtract(Range{Low: x, High: x})
	} else {
		t.Add(Range{Low: x, High: x})
	}
}

func (t *Tree) AddTree(nt *Tree) {
	if nt == nil {
		return
	}
	for r := range nt.RangeChan() {
		t.Add(r)
	}
}

func (t *Tree) SubtractTree(nt *Tree) {
	if nt == nil {
		return
	}
	for r := range nt.RangeChan() {
		t.Subtract(r)
	}
}

func (t *Tree) Invert(r Range) {
	for x := r.Low; x <= r.High; x++ {
		t.InvertInt(x)
	}
}

func (t *Tree) InvertTree(nt *Tree) {
	if nt == nil {
		return
	}
	for r := range nt.RangeChan() {
		t.Invert(r)
	}
}

func (t *Tree) FindOverlapping(r Range) []Range {
	ranges := []Range{}
	t.tree.AscendGreaterOrEqual(r, func(item llrb.Item) bool {
		ri := item.(Range)
		if ri.Low > r.High {
			return false
		}
		ranges = append(ranges, ri)
		return true
	})

	return ranges
}

func (t *Tree) Contains(x int) bool {
	if t == nil {
		return false
	}
	return !t.Find(x).IsZero()
}

func (t *Tree) Subtract(r Range) {
	overlapping := t.FindOverlapping(r)
	for _, o := range overlapping {
		t.tree.Delete(o)
		if o.Low < r.Low {
			t.tree.ReplaceOrInsert(Range{Low: o.Low, High: r.Low - 1})
		}
		if o.High > r.High {
			t.tree.ReplaceOrInsert(Range{Low: r.High + 1, High: o.High})
		}
	}
}

func (t *Tree) Slice() []Range {
	s := make([]Range, 0, t.tree.Len())
	t.tree.AscendGreaterOrEqual(Range{Low: 0, High: 0}, func(i llrb.Item) bool {
		s = append(s, i.(Range))
		return true
	})
	return s
}

func (t *Tree) RangeChan() <-chan Range {
	out := make(chan Range)
	go func() {
		t.tree.AscendGreaterOrEqual(Range{Low: 0, High: 0}, func(i llrb.Item) bool {
			out <- i.(Range)
			return true
		})
		close(out)
	}()
	return out
}

func (t *Tree) Print() {
	t.tree.AscendGreaterOrEqual(Range{Low: 0, High: 0}, func(i llrb.Item) bool {
		fmt.Printf("[%d, %d]\n", i.(Range).Low, i.(Range).High)
		return true
	})
}

func cloneNode(n *llrb.Node) *llrb.Node {
	if n == nil {
		return nil
	}
	clone := &llrb.Node{
		Item:  n.Item,
		Black: n.Black,
	}
	return clone
}

func (t *Tree) Clone() *Tree {
	node := t.tree.Root()
	stack := make([]*llrb.Node, 0)

	clone := New()
	ctree := *t.tree
	clone.tree = &ctree
	cstack := make([]*llrb.Node, 0)

	if node != nil {
		stack = append(stack, node)
		cnode := cloneNode(node)
		clone.tree.SetRoot(cnode)
		cstack = append(cstack, cnode)
	}

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		cnode := cstack[len(cstack)-1]
		stack = stack[:len(stack)-1]
		cstack = cstack[:len(cstack)-1]

		if node.Right != nil {
			rnode := cloneNode(node.Right)
			cnode.Right = rnode
			stack = append(stack, node.Right)
			cstack = append(cstack, rnode)
		}

		if node.Left != nil {
			rnode := cloneNode(node.Left)
			cnode.Left = rnode
			stack = append(stack, node.Left)
			cstack = append(cstack, rnode)
		}
	}

	return clone
}

func (t *Tree) Len() int {
	return t.tree.Len()
}
