package clip

import (
	"errors"
	"fmt"
	"io"

	"github.com/x448/float16"
)

var ErrMismatchedLength = errors.New("slice lengths do not match")

type Float float16.Float16

func (e Float) Float32() float32 {
	return float16.Float16(e).Float32()
}

func DotProductFloat32Float(a []float32, b []Float) (float32, error) {
	l := len(a)
	if l != len(b) {
		return 0, fmt.Errorf("slice lengths do not match, a %d b %d", l, len(b))
	}

	dot := float32(0)
	for i := 0; i < l; i++ {
		dot += a[i] * b[i].Float32()
	}
	return dot, nil
}

func DotProductFloat32Float32(a []float32, b []float32) (float32, error) {
	l := len(a)
	if l != len(b) {
		return 0, fmt.Errorf("slice lengths do not match, a %d b %d", l, len(b))
	}

	dot := float32(0)
	for i := 0; i < l; i++ {
		dot += a[i] * b[i]
	}
	return dot, nil
}

func CosineSimilarityEmbeddingFloat32(e Embedding, f []float32, invnorm float32) (float32, error) {
	dot, err := DotProductFloat32Float32(e.Float32(), f)
	if err != nil {
		return 0, err
	}
	return dot * invnorm * e.InvNormFloat32(), nil
}

func CosineSimilarityFloat32Float32(a []float32, ainvnorm float32, b []float32, binvnorm float32) (float32, error) {
	dot, err := DotProductFloat32Float32(a, b)
	if err != nil {
		return 0, err
	}
	return dot * ainvnorm * binvnorm, nil
}

// Most real world inverse vector norms of embeddings fall
// within ~500 of 11843, so it's more efficient to store
// the inverse vector norm as an offset of this number.
const InvNormMean = 11843

type Clip interface {
	EmbedImagePath(path string) (Embedding, error)
	EmbedImageReader(r io.Reader) (Embedding, error)
	EmbedText(text string) (Embedding, error)
}

type Embedding interface {
	Byte() []byte
	Float() []Float
	Float32() []float32
	InvNormUint16() uint16
	InvNormFloat32() float32
}
