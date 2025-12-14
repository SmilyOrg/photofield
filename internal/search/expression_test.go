package search

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/goccy/go-yaml"
)

type ExpressionTestCase struct {
	Search string     `yaml:"search"`
	Expr   Expression `yaml:"expr"`
	Error  string     `yaml:"error,omitempty"`
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

	var actualCases []ExpressionTestCase
	for i, tc := range testCases {
		t.Run(tc.Search, func(t *testing.T) {
			// Parse the query
			query, err := Parse(tc.Search)
			if err != nil {
				t.Fatalf("Test case %d: Failed to parse query '%s': %v", i, tc.Search, err)
			}

			expected := tc.Expr
			actual, err := query.Expression()

			// Handle expected errors
			if tc.Error != "" {
				if err == nil {
					t.Errorf("Test case %d: Expected an error but got none", i)
				}
				if tc.Error != "" && err != nil && !strings.Contains(err.Error(), tc.Error) {
					t.Errorf("Test case %d: Expected error message to contain %q, got %q", i, tc.Error, err.Error())
				}
			} else if err != nil {
				t.Fatalf("Test case %d: Failed to get expression for query '%s': %v", i, tc.Search, err)
			}

			actualCases = append(actualCases, ExpressionTestCase{
				Search: tc.Search,
				Expr:   actual,
				Error:  tc.Error,
			})

			expectedStr, err := yaml.Marshal(expected)
			if err != nil {
				t.Fatalf("Test case %d: Failed to marshal expected expression: %v", i, err)
			}
			actualStr, err := yaml.Marshal(actual)
			if err != nil {
				t.Fatalf("Test case %d: Failed to marshal actual expression: %v", i, err)
			}

			println("--- EXPECTED ---")
			println(string(expectedStr))
			println("--- ACTUAL ---")
			println(string(actualStr))

			assert.Equal(t, string(expectedStr), string(actualStr), "Test case %d: Expression mismatch\n--- SEARCH ---\n%s\n\n--- EXPECTED ---\n%s\n--- ACTUAL ---\n%s\n--- DIFF ---", i, tc.Search, string(expectedStr), string(actualStr))

			// Check that there are no unexpected errors in the expression
			if tc.Error == "" && len(expected.Errors) == 0 && len(actual.Errors) > 0 {
				for _, fieldErr := range actual.Errors {
					t.Errorf("Test case %d: Unexpected field %q error: %v", i, fieldErr.Name, fieldErr.Error)
				}
			}
		})
	}

	if t.Failed() {
		// Output the actual cases for easier test case updates
		actualData, err := yaml.Marshal(actualCases)
		if err != nil {
			t.Fatalf("Failed to marshal actual test cases: %v", err)
		}
		println()
		println("------------------------")
		println("--- expressions.yaml ---")
		println("------------------------")
		println(strings.ReplaceAll(string(actualData), "\n- ", "\n\n- "))
		println("------------------------")
		println()
	}

}
