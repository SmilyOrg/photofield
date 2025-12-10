package search

import (
	"fmt"
	"strconv"
)

type Int64 struct {
	FieldMeta `json:"meta,omitempty"`
	Value     int64 `json:"value,omitempty"`
}

func (q *Query) ExpressionInt(key string) (f Int64) {
	values := q.QualifierTerms(key)
	if len(values) == 0 {
		return
	}

	value := values[0]
	f.Present = true
	f.Name = key

	v, err := strconv.ParseInt(value.Qualifier.Value, 10, 64)
	if err != nil {
		f.Error = fmt.Errorf("invalid number: %w", err)
		return
	}
	f.Value = int64(v)
	f.Token = value.Token()
	return
}
