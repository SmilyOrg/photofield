package dag

import (
	"photofield/internal/image"
)

type Id = image.ImageId
type Index = int

type Photo struct {
	Id          Id
	AspectRatio float32
	Aux         bool
}

type Aux struct {
	Text string
}

type Node struct {
	ShortestParent Index
	Cost           float32
	TotalAspect    float32
}
