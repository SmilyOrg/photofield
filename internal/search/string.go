package search

import (
	"fmt"
	"strings"
)

type String struct {
	FieldMeta `json:"meta,omitempty"`
	Value     string `json:"value,omitempty"`
}

type Strings []String

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

func (q *Query) ExpressionStrings(key string) (f Strings) {
	values := q.QualifierTerms(key)
	if len(values) == 0 {
		return
	}

	for _, value := range values {
		s := String{}
		s.Present = true
		s.Name = key
		s.Value = value.Qualifier.Value
		s.Token = value.Token()
		if s.Value == "" {
			s.Error = fmt.Errorf("value cannot be empty")
		}
		f = append(f, s)
	}
	return
}

func (f Strings) Values() []string {
	var values []string
	for _, s := range f {
		values = append(values, s.Value)
	}
	return values
}
