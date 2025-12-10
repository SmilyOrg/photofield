package search

import (
	"fmt"
)

// FieldMeta contains metadata about a parsed field
type FieldMeta struct {
	// Name
	Name string `json:"name,omitempty"`

	// Source token from the Query AST with position info
	Token Token `json:"token,omitempty"`

	// Parse error if any (field may still have default/partial value)
	Error error `json:"error,omitempty"`

	// Whether the field was explicitly set in the query
	Present bool `json:"present,omitempty"`
}

// Expression represents a validated and typed search query
type Expression struct {
	query       *Query
	Created     DateRange `json:"created,omitempty"`
	Threshold   Float32   `json:"t,omitempty"`
	Deduplicate Float32   `json:"dedup,omitempty"`

	// Aggregate errors for convenient iteration
	Errors []FieldMeta `json:"errors,omitempty"`
}

// Expression validates the query and returns a typed Expression.
func (q *Query) Expression() (Expression, error) {
	if q == nil {
		return Expression{}, nil
	}

	expr := Expression{
		query: q,
	}

	expr.Created = q.ExpressionDateRange("created")
	expr.addFieldError(expr.Created.FieldMeta)

	expr.Threshold = q.ExpressionFloat32("t")
	expr.addFieldError(expr.Threshold.FieldMeta)

	expr.Deduplicate = q.ExpressionFloat32("dedup")
	expr.addFieldError(expr.Deduplicate.FieldMeta)

	var err error
	if len(expr.Errors) > 0 {
		more := ""
		if len(expr.Errors) > 1 {
			more = fmt.Sprintf(" (+%d more)", len(expr.Errors)-1)
		}
		err = fmt.Errorf("expression error: %w%s", expr.Errors[0].Error, more)
	}
	return expr, err
}

func (expr *Expression) addFieldError(meta FieldMeta) {
	if meta.Error == nil || meta.Error == ErrNotFound {
		return
	}
	expr.Errors = append(expr.Errors, meta)
}
