package search

import (
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestTokens(t *testing.T) {
	query, err := Parse(`hello world tag:foo "a string" NOT bar`)
	if err != nil {
		t.Fatal(err)
	}

	tokens := query.Tokens()
	assert.Equal(t, 5, len(tokens))

	// First token: "hello"
	assert.Equal(t, "word", tokens[0].Type)
	assert.Equal(t, "hello", tokens[0].Value)
	assert.Equal(t, false, tokens[0].Not)

	// Second token: "world"
	assert.Equal(t, "word", tokens[1].Type)
	assert.Equal(t, "world", tokens[1].Value)
	assert.Equal(t, false, tokens[1].Not)

	// Third token: "tag:foo"
	assert.Equal(t, "qualifier", tokens[2].Type)
	assert.Equal(t, "tag:foo", tokens[2].Value)
	assert.Equal(t, "tag", tokens[2].Key)
	assert.Equal(t, "foo", tokens[2].QualVal)
	assert.Equal(t, false, tokens[2].Not)

	// Fourth token: "a string"
	assert.Equal(t, "string", tokens[3].Type)
	assert.Equal(t, "a string", tokens[3].Value)
	assert.Equal(t, false, tokens[3].Not)

	// Fifth token: "bar" with NOT
	assert.Equal(t, "word", tokens[4].Type)
	assert.Equal(t, "bar", tokens[4].Value)
	assert.Equal(t, true, tokens[4].Not)
}

func TestTokensWithQualifiers(t *testing.T) {
	query, err := Parse(`created:2022-01-01..2022-12-31 k:5 bias:0.01`)
	if err != nil {
		t.Fatal(err)
	}

	tokens := query.Tokens()
	assert.Equal(t, 3, len(tokens))

	assert.Equal(t, "qualifier", tokens[0].Type)
	assert.Equal(t, "created", tokens[0].Key)
	assert.Equal(t, "2022-01-01..2022-12-31", tokens[0].QualVal)
	assert.Equal(t, "created:2022-01-01..2022-12-31", tokens[0].Value)

	assert.Equal(t, "qualifier", tokens[1].Type)
	assert.Equal(t, "k", tokens[1].Key)
	assert.Equal(t, "5", tokens[1].QualVal)

	assert.Equal(t, "qualifier", tokens[2].Type)
	assert.Equal(t, "bias", tokens[2].Key)
	assert.Equal(t, "0.01", tokens[2].QualVal)
}

func TestTokensEmpty(t *testing.T) {
	query, err := Parse(``)
	if err != nil {
		t.Fatal(err)
	}

	tokens := query.Tokens()
	assert.Equal(t, 0, len(tokens))
}

func TestTokensNil(t *testing.T) {
	var query *Query
	tokens := query.Tokens()
	assert.Equal(t, []Token(nil), tokens)
}

func TestTokenPositions(t *testing.T) {
	query, err := Parse(`hello tag:world "test string"`)
	if err != nil {
		t.Fatal(err)
	}

	tokens := query.Tokens()

	// Verify positions are captured (offsets are 0-indexed)
	assert.Equal(t, 0, tokens[0].Start) // "hello" starts at offset 0
	assert.Equal(t, 5, tokens[0].End)   // "hello" ends at offset 5

	assert.Equal(t, 6, tokens[1].Start) // "tag:world" starts at offset 6
	assert.Equal(t, 15, tokens[1].End)  // "tag:world" ends at offset 15

	assert.Equal(t, 16, tokens[2].Start) // "test string" starts at offset 16
	assert.Equal(t, 29, tokens[2].End)   // "test string" ends at offset 29
}

// parseTokenTestFile parses a token test file with ASCII art annotations.
// Format:
//
//	Line 1: Query text
//	Line 2: ^---- marker showing token span
//	Line 3: value (type) [NOT]
//	Empty line separates test cases
func parseTokenTestFile(content string) []struct {
	name   string
	query  string
	tokens []struct {
		value       string
		typ         string
		not         bool
		startOffset int
		endOffset   int
	}
} {
	var testCases []struct {
		name   string
		query  string
		tokens []struct {
			value       string
			typ         string
			not         bool
			startOffset int
			endOffset   int
		}
	}

	lines := strings.Split(content, "\n")
	i := 0

	for i < len(lines) {
		// Skip comments and empty lines at start
		for i < len(lines) && (strings.HasPrefix(strings.TrimSpace(lines[i]), "#") || strings.TrimSpace(lines[i]) == "") {
			i++
		}
		if i >= len(lines) {
			break
		}

		// Read query line
		query := lines[i]
		i++

		var tokens []struct {
			value       string
			typ         string
			not         bool
			startOffset int
			endOffset   int
		}

		// Read token annotations until empty line
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
			markerLine := lines[i]
			i++

			// Find ^ marker position
			caretPos := strings.Index(markerLine, "^")
			if caretPos == -1 {
				continue
			}

			// Find the dash sequence to determine end position
			dashStart := caretPos + 1
			dashEnd := dashStart
			for dashEnd < len(markerLine) && markerLine[dashEnd] == '-' {
				dashEnd++
			}

			startOffset := caretPos // 0-indexed offset
			endOffset := dashEnd    // 0-indexed offset

			// Read the next line for annotation: value (type) [NOT]
			if i >= len(lines) {
				break
			}
			annotation := strings.TrimSpace(lines[i])
			i++

			// Extract value and type from "value (type)" or "value (type) NOT"
			parenIdx := strings.Index(annotation, "(")
			if parenIdx == -1 {
				continue
			}

			value := strings.TrimSpace(annotation[:parenIdx])
			rest := annotation[parenIdx+1:]

			closeParenIdx := strings.Index(rest, ")")
			if closeParenIdx == -1 {
				continue
			}

			typ := strings.TrimSpace(rest[:closeParenIdx])
			not := strings.Contains(rest[closeParenIdx:], "NOT")

			tokens = append(tokens, struct {
				value       string
				typ         string
				not         bool
				startOffset int
				endOffset   int
			}{value, typ, not, startOffset, endOffset})
		}

		if query != "" {
			testCases = append(testCases, struct {
				name   string
				query  string
				tokens []struct {
					value       string
					typ         string
					not         bool
					startOffset int
					endOffset   int
				}
			}{
				name:   query,
				query:  query,
				tokens: tokens,
			})
		}

		i++ // Skip empty line separator
	}

	return testCases
}

// TestTokenPositionsFromFile reads test cases from an external file with ASCII annotations
func TestTokenPositionsFromFile(t *testing.T) {
	content, err := os.ReadFile("testdata/token_positions.txt")
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	testCases := parseTokenTestFile(string(content))

	if len(testCases) == 0 {
		t.Fatal("No test cases found in file")
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsedQuery, err := Parse(tc.query)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			tokens := parsedQuery.Tokens()
			if len(tokens) != len(tc.tokens) {
				t.Errorf("Expected %d tokens, got %d\nQuery: %s", len(tc.tokens), len(tokens), tc.query)
				for i, tok := range tokens {
					t.Logf("  Token %d: %q (type=%s, start=%d, end=%d)",
						i, tok.Value, tok.Type, tok.Start, tok.End)
				}
				return
			} // Verify each token
			for i, expected := range tc.tokens {
				token := tokens[i]

				if token.Start != expected.startOffset {
					t.Errorf("Token %d (%q): expected start offset %d, got %d",
						i, expected.value, expected.startOffset, token.Start)
				}

				if token.End != expected.endOffset {
					t.Errorf("Token %d (%q): expected end offset %d, got %d",
						i, expected.value, expected.endOffset, token.End)
				}

				if token.Value != expected.value {
					t.Errorf("Token %d: expected value %q, got %q",
						i, expected.value, token.Value)
				}

				if token.Type != expected.typ {
					t.Errorf("Token %d: expected type %q, got %q",
						i, expected.typ, token.Type)
				}

				if token.Not != expected.not {
					t.Errorf("Token %d: expected not=%v, got %v",
						i, expected.not, token.Not)
				}
			}
		})
	}
}
