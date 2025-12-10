package search

import (
	"os"
	"testing"

	"github.com/goccy/go-yaml"
)

type ExpressionTestCase struct {
	Search string     `yaml:"search"`
	Expr   Expression `yaml:"expr"`
}

func TestExpression(t *testing.T) {
	data, err := os.ReadFile("testdata/expressions.yaml")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	var testCases []ExpressionTestCase
	if err := yaml.Unmarshal(data, &testCases); err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	for i, tc := range testCases {
		t.Run(tc.Search, func(t *testing.T) {
			// Parse the query
			query, err := Parse(tc.Search)
			if err != nil {
				t.Fatalf("Test case %d: Failed to parse query '%s': %v", i, tc.Search, err)
			}

			// Get the expression
			expr, err := query.Expression()
			if err != nil {
				t.Fatalf("Test case %d: Failed to get expression for query '%s': %v", i, tc.Search, err)
			}

			// Check created field if specified in test case
			if !tc.Expr.Created.From.IsZero() || !tc.Expr.Created.To.IsZero() {
				checkDateRange(t, i, "Created", expr.Created, tc.Expr.Created)
			}

			// Check that there are no errors in the expression
			if len(expr.Errors) > 0 {
				for _, fieldErr := range expr.Errors {
					t.Errorf("Test case %d: Field %q error: %v", i, fieldErr.Name, fieldErr.Error)
				}
			}
		})
	}
}

func checkMeta(t *testing.T, testNum int, fieldName string, actual, expected FieldMeta) {
	t.Helper()

	if expected.Name != "" && actual.Name != expected.Name {
		t.Errorf("Test case %d: %s.Name = %q, want %q", testNum, fieldName, actual.Name, expected.Name)
	}

	if expected.Token.Type != "" && actual.Token.Type != expected.Token.Type {
		t.Errorf("Test case %d: %s.Token.Type = %q, want %q", testNum, fieldName, actual.Token.Type, expected.Token.Type)
	}

	if expected.Token.Value != "" && actual.Token.Value != expected.Token.Value {
		t.Errorf("Test case %d: %s.Token.Value = %q, want %q", testNum, fieldName, actual.Token.Value, expected.Token.Value)
	}

	if actual.Error != nil && expected.Error == nil {
		t.Errorf("Test case %d: %s unexpected error: %v", testNum, fieldName, actual.Error)
	}

	if actual.Error == nil && expected.Error != nil {
		t.Errorf("Test case %d: %s expected error but got none", testNum, fieldName)
	}
}

func checkDateRange(t *testing.T, testNum int, fieldName string, actual, expected DateRange) {
	t.Helper()

	if !actual.From.Equal(expected.From) {
		t.Errorf("Test case %d: %s.From = %v, want %v", testNum, fieldName, actual.From, expected.From)
	}

	if !actual.To.Equal(expected.To) {
		t.Errorf("Test case %d: %s.To = %v, want %v", testNum, fieldName, actual.To, expected.To)
	}

	if actual.FromWildcard != expected.FromWildcard {
		t.Errorf("Test case %d: %s.FromWildcard = %v, want %v", testNum, fieldName, actual.FromWildcard, expected.FromWildcard)
	}

	if actual.ToWildcard != expected.ToWildcard {
		t.Errorf("Test case %d: %s.ToWildcard = %v, want %v", testNum, fieldName, actual.ToWildcard, expected.ToWildcard)
	}

	checkMeta(t, testNum, fieldName, actual.FieldMeta, expected.FieldMeta)
}
