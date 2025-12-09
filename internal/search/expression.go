package search

// FieldMeta contains metadata about a parsed field
type FieldMeta struct {
	// Name
	Name string `json:"name"`

	// Source token from the Query AST with position info
	Token Token `json:"token"`

	// Parse error if any (field may still have default/partial value)
	Error error `json:"error"`

	// Whether the field was explicitly set in the query
	Present bool `json:"present"`
}

// Expression represents a validated and typed search query
type Expression struct {
	query   *Query
	Created DateRange `json:"created"`

	// Aggregate errors for convenient iteration
	Errors []FieldMeta `json:"errors"`

	// Future fields:
	// Tags                []string
	// SimilarityThreshold float32
	// SimilarImage        int64 // ImageId
	// DeduplicateAt       float32
	// Strategy            string
	// KNN_K               int
	// KNN_Bias            float32
	// Words               string
}

// Expression validates the query and returns a typed Expression.
func (q *Query) Expression() (*Expression, error) {
	if q == nil {
		return nil, ErrNilQuery
	}

	expr := &Expression{
		query: q,
	}

	// Validate and parse "created" qualifier
	// from, to, err := q.QualifierDateRange("created")
	expr.Created = q.ExpressionDateRange("created")
	expr.addFieldError(expr.Created.FieldMeta)

	return expr, nil
}

func (expr *Expression) addFieldError(meta FieldMeta) {
	if meta.Error == nil || meta.Error == ErrNotFound {
		return
	}
	expr.Errors = append(expr.Errors, meta)
}
