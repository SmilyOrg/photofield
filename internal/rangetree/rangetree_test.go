package rangetree

import (
	"math/rand"
	"testing"
)

func assertRangeSlice(t *testing.T, expected []Range, actual []Range) {
	if len(actual) != len(expected) {
		t.Errorf("length mismatch: expected %d, got %d", len(expected), len(actual))
		for i := range expected {
			if i >= len(actual) {
				t.Errorf("  expected %s, got nothing", expected[i])
				continue
			}
			t.Errorf("  expected %s, got %s", expected[i], actual[i])
		}
		return
	}
	for i := range actual {
		if actual[i].Low != expected[i].Low {
			t.Errorf("expected %d, got %d", expected[i].Low, actual[i].Low)
		}
		if actual[i].High != expected[i].High {
			t.Errorf("expected %d, got %d", expected[i].High, actual[i].High)
		}
	}
}

func TestAddToEmpty(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	if rt.tree.Len() != 1 {
		t.Errorf("expected 1, got %d", rt.tree.Len())
	}
	if rt.tree.Min().(Range).Low != 1 {
		t.Errorf("expected 1, got %d", rt.tree.Min().(Range).Low)
	}
	if rt.tree.Min().(Range).High != 4 {
		t.Errorf("expected 4, got %d", rt.tree.Min().(Range).High)
	}
	if rt.tree.Max().(Range).Low != 1 {
		t.Errorf("expected 1, got %d", rt.tree.Max().(Range).Low)
	}
	if rt.tree.Max().(Range).High != 4 {
		t.Errorf("expected 4, got %d", rt.tree.Max().(Range).High)
	}
	for i := 1; i <= 4; i++ {
		r := rt.Find(i)
		if r.Low != 1 {
			t.Errorf("expected 1, got %d", r.Low)
		}
		if r.High != 4 {
			t.Errorf("expected 4, got %d", r.High)
		}
	}
	for i := 5; i <= 10; i++ {
		r := rt.Find(i)
		if !r.IsZero() {
			t.Errorf("expected empty, got %d", r.Low)
		}
	}
	expected := []Range{{Low: 1, High: 4}}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddSeparate(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	rt.Add(Range{Low: 6, High: 8})
	expected := []Range{
		{Low: 1, High: 4},
		{Low: 6, High: 8},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddConsecutive(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 2, High: 2})
	rt.Add(Range{Low: 3, High: 3})
	rt.Add(Range{Low: 1, High: 1})
	expected := []Range{
		{Low: 1, High: 3},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddLow(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 5, High: 7})
	rt.Add(Range{Low: 7, High: 9})
	expected := []Range{
		{Low: 5, High: 9},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddHigh(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 5, High: 7})
	rt.Add(Range{Low: 3, High: 5})
	expected := []Range{
		{Low: 3, High: 7},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddOutside(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 5, High: 7})
	rt.Add(Range{Low: 3, High: 9})
	expected := []Range{
		{Low: 3, High: 9},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddInside(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 3, High: 8})
	rt.Add(Range{Low: 5, High: 6})
	expected := []Range{
		{Low: 3, High: 8},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddMultiple(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	rt.Add(Range{Low: 3, High: 6})
	rt.Add(Range{Low: 5, High: 8})
	rt.Add(Range{Low: 7, High: 10})
	expected := []Range{
		{Low: 1, High: 10},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddAdjacentLow(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	rt.Add(Range{Low: 5, High: 6})
	expected := []Range{
		{Low: 1, High: 6},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddAdjacentHigh(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 4, High: 6})
	rt.Add(Range{Low: 1, High: 3})
	expected := []Range{
		{Low: 1, High: 6},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddAdjacentMultiple(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	rt.Add(Range{Low: 5, High: 6})
	rt.Add(Range{Low: 7, High: 8})
	expected := []Range{
		{Low: 1, High: 8},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddSingleGapLow(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	rt.Add(Range{Low: 6, High: 8})
	rt.Add(Range{Low: 10, High: 12})
	expected := []Range{
		{Low: 1, High: 4},
		{Low: 6, High: 8},
		{Low: 10, High: 12},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddSingleGapHigh(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 9, High: 12})
	rt.Add(Range{Low: 5, High: 7})
	rt.Add(Range{Low: 1, High: 3})
	expected := []Range{
		{Low: 1, High: 3},
		{Low: 5, High: 7},
		{Low: 9, High: 12},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddSame(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 4})
	rt.Add(Range{Low: 1, High: 4})
	expected := []Range{
		{Low: 1, High: 4},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddMultipleOverlapping(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Add(Range{Low: 3, High: 6})
	rt.Add(Range{Low: 5, High: 8})
	rt.Add(Range{Low: 7, High: 12})
	expected := []Range{
		{Low: 1, High: 12},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestAddIntRandom(t *testing.T) {
	rt := New()
	seed := int64(1)
	min := 1
	max := 2000
	count := 1000
	rnd := rand.NewSource(seed)
	expected := make([]bool, max-min)
	for i := 0; i < count; i++ {
		n := min + int(rnd.Int63())%max
		expected[n-min] = true
		rt.AddInt(n)
	}
	t.Logf("testing %v numbers %v-%v, tree size %v", count, min, max, rt.Len())
	for i := 0; i < len(expected); i++ {
		n := min + i
		if expected[i] != rt.Contains(n) {
			not := "not "
			if expected[i] {
				not = ""
			}
			t.Errorf("expected %v to be %scontained in tree", n, not)
		}
	}
}

func TestFindOverlappingTwo(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 4, High: 6})
	rt.Add(Range{Low: 8, High: 10})
	rt.Add(Range{Low: 12, High: 14})

	expected := []Range{
		{Low: 4, High: 6},
		{Low: 8, High: 10},
	}

	actual := rt.FindOverlapping(Range{Low: 5, High: 9})
	assertRangeSlice(t, expected, actual)
}

func TestFindOverlappingSingle(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 4, High: 6})
	rt.Add(Range{Low: 8, High: 10})
	rt.Add(Range{Low: 12, High: 14})

	expected := []Range{
		{Low: 4, High: 6},
	}

	actual := rt.FindOverlapping(Range{Low: 5, High: 5})
	assertRangeSlice(t, expected, actual)
}

func TestFindOverlappingNone(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 3, High: 5})
	rt.Add(Range{Low: 6, High: 8})
	rt.Add(Range{Low: 10, High: 12})

	expected := []Range{}

	actual := rt.FindOverlapping(Range{Low: 13, High: 15})
	assertRangeSlice(t, expected, actual)
}

func TestFindOverlappingAll(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 4, High: 6})
	rt.Add(Range{Low: 8, High: 10})
	rt.Add(Range{Low: 12, High: 14})

	expected := []Range{
		{Low: 1, High: 2},
		{Low: 4, High: 6},
		{Low: 8, High: 10},
		{Low: 12, High: 14},
	}

	actual := rt.FindOverlapping(Range{Low: 0, High: 15})
	assertRangeSlice(t, expected, actual)
}

func TestFindOverlappingAllExact(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 4, High: 6})
	rt.Add(Range{Low: 8, High: 10})
	rt.Add(Range{Low: 12, High: 14})

	expected := []Range{
		{Low: 1, High: 2},
		{Low: 4, High: 6},
		{Low: 8, High: 10},
		{Low: 12, High: 14},
	}

	actual := rt.FindOverlapping(Range{Low: 1, High: 14})
	assertRangeSlice(t, expected, actual)
}

func TestFindOverlappingLow(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 4, High: 6})
	rt.Add(Range{Low: 8, High: 10})
	rt.Add(Range{Low: 12, High: 14})

	expected := []Range{
		{Low: 1, High: 2},
		{Low: 4, High: 6},
	}

	actual := rt.FindOverlapping(Range{Low: 1, High: 4})
	assertRangeSlice(t, expected, actual)
}

func TestFindOverlappingHigh(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 2})
	rt.Add(Range{Low: 3, High: 4})
	rt.Add(Range{Low: 6, High: 8})
	rt.Add(Range{Low: 10, High: 12})

	expected := []Range{
		{Low: 6, High: 8},
		{Low: 10, High: 12},
	}

	actual := rt.FindOverlapping(Range{Low: 6, High: 12})
	assertRangeSlice(t, expected, actual)
}

func TestSubtract(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Subtract(Range{Low: 4, High: 6})
	expected := []Range{
		{Low: 1, High: 3},
		{Low: 7, High: 10},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestSubtractAdjacent(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 3})
	rt.Add(Range{Low: 7, High: 10})
	rt.Subtract(Range{Low: 4, High: 6})
	expected := []Range{
		{Low: 1, High: 3},
		{Low: 7, High: 10},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting a non-overlapping interval from an empty tree: The tree should remain unchanged.
func TestSubtractNonOverlappingEmpty(t *testing.T) {
	rt := New()
	rt.Subtract(Range{Low: 4, High: 6})
	assertRangeSlice(t, []Range{}, rt.Slice())
}

// Subtracting a non-overlapping interval from a tree with a single interval: The tree should remain unchanged.
func TestSubtractNonOverlappingSingle(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Subtract(Range{Low: 11, High: 12})
	expected := []Range{
		{Low: 1, High: 10},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting an interval that completely covers the only interval in the tree: The tree should become empty.
func TestSubtractCoversSingle(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Subtract(Range{Low: 0, High: 11})
	assertRangeSlice(t, []Range{}, rt.Slice())
}

// Subtracting an interval that covers the beginning of the only interval in the tree: The tree should have a single interval with a modified lower bound.
func TestSubtractCoversBeginningSingle(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Subtract(Range{Low: 0, High: 5})
	expected := []Range{
		{Low: 6, High: 10},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting an interval that covers the end of the only interval in the tree: The tree should have a single interval with a modified upper bound.
func TestSubtractCoversEndSingle(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Subtract(Range{Low: 5, High: 11})
	expected := []Range{
		{Low: 1, High: 4},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting an interval that is completely within the only interval in the tree: The tree should have two new intervals.
func TestSubtractWithinSingle(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Subtract(Range{Low: 3, High: 7})
	expected := []Range{
		{Low: 1, High: 2},
		{Low: 8, High: 10},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting an interval that overlaps with multiple intervals in the tree: The tree should have updated intervals with modified boundaries.
func TestSubtractOverlappingMultiple(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Add(Range{Low: 15, High: 20})
	rt.Add(Range{Low: 25, High: 30})
	rt.Add(Range{Low: 35, High: 40})
	rt.Subtract(Range{Low: 5, High: 35})
	expected := []Range{
		{Low: 1, High: 4},
		{Low: 36, High: 40},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting an interval that covers the entire tree: The tree should become empty.
func TestSubtractCoversAll(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Add(Range{Low: 15, High: 20})
	rt.Add(Range{Low: 25, High: 30})
	rt.Add(Range{Low: 35, High: 40})
	rt.Subtract(Range{Low: 0, High: 41})
	assertRangeSlice(t, []Range{}, rt.Slice())
}

// Subtracting an interval that is partially outside the tree: The tree should have updated intervals with modified boundaries.
func TestSubtractPartialOutside(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Add(Range{Low: 15, High: 20})
	rt.Add(Range{Low: 25, High: 30})
	rt.Add(Range{Low: 35, High: 40})
	rt.Subtract(Range{Low: 5, High: 45})
	expected := []Range{
		{Low: 1, High: 4},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

// Subtracting an interval that is completely outside the tree: The tree should remain unchanged.
func TestSubtractCompletelyOutside(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 1, High: 10})
	rt.Add(Range{Low: 15, High: 20})
	rt.Add(Range{Low: 25, High: 30})
	rt.Add(Range{Low: 35, High: 40})
	rt.Subtract(Range{Low: 45, High: 50})
	expected := []Range{
		{Low: 1, High: 10},
		{Low: 15, High: 20},
		{Low: 25, High: 30},
		{Low: 35, High: 40},
	}
	assertRangeSlice(t, expected, rt.Slice())
}

func TestContains(t *testing.T) {
	rt := New()
	rt.Add(Range{Low: 3, High: 5})
	rt.Add(Range{Low: 6, High: 8})
	rt.Add(Range{Low: 10, High: 12})
	rt.Add(Range{Low: 15, High: 20})
	cases := []struct {
		x        int
		contains bool
	}{
		{1, false},
		{2, false},
		{3, true},
		{4, true},
		{5, true},
		{6, true},
		{7, true},
		{8, true},
		{9, false},
		{10, true},
		{11, true},
		{12, true},
		{13, false},
		{14, false},
		{15, true},
		{16, true},
		{17, true},
		{18, true},
		{19, true},
		{20, true},
		{21, false},
	}
	for _, c := range cases {
		if rt.Contains(c.x) != c.contains {
			t.Errorf("Contains(%d) = %t, want %t", c.x, !c.contains, c.contains)
		}
	}
}

func TestClone(t *testing.T) {
	a := New()
	a.Add(Range{Low: 3, High: 5})
	a.Add(Range{Low: 7, High: 8})
	a.Add(Range{Low: 10, High: 12})
	a.Add(Range{Low: 15, High: 20})

	a1 := a.Slice()

	b := a.Clone()
	assertRangeSlice(t, a1, b.Slice())

	a2 := a.Slice()
	assertRangeSlice(t, a1, a2)

	b.Subtract(Range{Low: 7, High: 10})
	a3 := a.Slice()
	assertRangeSlice(t, a1, a3)

	expected := []Range{
		{Low: 3, High: 5},
		{Low: 11, High: 12},
		{Low: 15, High: 20},
	}
	assertRangeSlice(t, expected, b.Slice())
}
