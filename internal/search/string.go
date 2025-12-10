package search

import (
	"fmt"
	"strings"
)

type String struct {
	FieldMeta `json:"meta,omitempty"`
	Value     string `json:"value,omitempty"`
}

func (q *Query) ExpressionEnum(key string, validValues []string) (f String) {
	values := q.QualifierTerms(key)
	if len(values) == 0 {
		return
	}

	value := values[0]
	f.Present = true
	f.Name = key

	f.Value = value.Qualifier.Value
	f.Token = value.Token()
	for _, vv := range validValues {
		if f.Value == vv {
			return
		}
	}
	f.Error = fmt.Errorf("unsupported value: %s (use %s)", f.Value, strings.Join(validValues, ", "))
	return
}
