package search

import (
	"fmt"
	"strconv"
)

type Float32 struct {
	FieldMeta `json:"meta,omitempty"`
	Value     float32 `json:"value,omitempty"`
}

func (q *Query) ExpressionFloat32(key string) (f Float32) {
	values := q.QualifierTerms(key)
	if len(values) == 0 {
		return
	}

	value := values[0]
	f.Present = true
	f.Name = key

	v, err := strconv.ParseFloat(value.Qualifier.Value, 32)
	if err != nil {
		f.Error = fmt.Errorf("invalid number: %w", err)
		return
	}
	f.Value = float32(v)
	f.Token = value.Token()
	return
}
