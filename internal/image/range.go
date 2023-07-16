package image

import "photofield/rangetree"

type Ids = *rangetree.Tree
type IdRange = rangetree.Range

func NewIds() Ids {
	return rangetree.New()
}

// Returns a range from low to high (inclusive)
func IdFromTo(low, high int) IdRange {
	return rangetree.FromTo(low, high)
}
