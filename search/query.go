package search

import (
	"fmt"
	"strconv"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Query struct {
	Terms []*Term `parser:"@@*" json:"terms"`
}

type Term struct {
	String    *string        `parser:"@String" json:"string,omitempty"`
	Qualifier *Qualifier     `parser:"| @@" json:"qualifier,omitempty"`
	Word      *string        `parser:"| @Word" json:"word,omitempty"`
	Pos       lexer.Position `parser:"" json:"start"`
	EndPos    lexer.Position `parser:"" json:"end"`
}

type Qualifier struct {
	Key   string `parser:"@Word ':'"`
	Value string `parser:"@Word (@':' @Word)*"`
}

var lex *lexer.StatefulDefinition
var par *participle.Parser[Query]

func init() {
	lex = lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Whitespace", Pattern: `[ \t]+`},
		{Name: "Word", Pattern: `[^\s:]+`},
		{Name: "String", Pattern: `"(\\"|[^"])*"`},
		{Name: "Colon", Pattern: `:`},
	})

	par = participle.MustBuild[Query](
		participle.Lexer(lex),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)
}

func Parse(str string) (*Query, error) {
	return par.ParseString("", str)
}

func (q *Query) QualifierInt(key string) (int, error) {
	if q == nil {
		return 0, fmt.Errorf("nil query")
	}

	if len(q.Terms) == 0 {
		return 0, fmt.Errorf("empty query")
	}

	if len(q.Terms) > 1 {
		return 0, fmt.Errorf("too many terms")
	}

	if q.Terms[0].Qualifier == nil {
		return 0, fmt.Errorf("no qualifier")
	}

	if q.Terms[0].Qualifier.Key != key {
		return 0, fmt.Errorf(`qualifier not %s`, key)
	}

	return strconv.Atoi(q.Terms[0].Qualifier.Value)
}

func (q *Query) QualifierValues(key string) []string {
	if q == nil {
		return nil
	}
	var values []string
	for _, term := range q.Terms {
		if term.Qualifier != nil && term.Qualifier.Key == key {
			values = append(values, term.Qualifier.Value)
		}
	}
	return values
}
