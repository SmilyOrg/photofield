package search

import (
	"fmt"
	"os"
	"strconv"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Query struct {
	Terms []*Term `parser:"@@*" json:"terms"`
}

type Term struct {
	Not       bool           `parser:"@'NOT'?" json:"not,omitempty"`
	String    *string        `parser:"(@String" json:"string,omitempty"`
	Qualifier *Qualifier     `parser:"| @@" json:"qualifier,omitempty"`
	Word      *string        `parser:"| @Word)" json:"word,omitempty"`
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
		{Name: "String", Pattern: `"(\\"|[^"])*"`},
		{Name: "Word", Pattern: `[^\s:]+`},
		{Name: "Colon", Pattern: `:`},
		{Name: "Whitespace", Pattern: `[ \t]+`},
	})
	par = participle.MustBuild[Query](
		participle.Lexer(lex),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
		participle.UseLookahead(2),
	)
}

func PrintTokens(str string) {
	l, err := lex.LexString("", str)
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		tok, err := l.Next()
		if err != nil {
			fmt.Println(err)
			return
		}
		if tok.EOF() {
			break
		}
		fmt.Println(tok.GoString())
	}
}

func Parse(str string) (*Query, error) {
	return par.ParseString("", str)
}

func ParseDebug(str string) (*Query, error) {
	PrintTokens(str)
	return par.ParseString("", str, participle.Trace(os.Stdout))
}

func (q *Query) QualifierInt(key string) (int, error) {
	if q == nil {
		return 0, fmt.Errorf("nil query")
	}

	if len(q.Terms) == 0 {
		return 0, fmt.Errorf("empty query")
	}

	for _, term := range q.Terms {
		if term.Qualifier != nil && term.Qualifier.Key == key {
			return strconv.Atoi(term.Qualifier.Value)
		}
	}
	return 0, fmt.Errorf("no qualifier")
}

func (q *Query) QualifierFloat32(key string) (float32, error) {
	if q == nil {
		return 0, fmt.Errorf("nil query")
	}

	if len(q.Terms) == 0 {
		return 0, fmt.Errorf("empty query")
	}

	for _, term := range q.Terms {
		if term.Qualifier != nil && term.Qualifier.Key == key {
			f, err := strconv.ParseFloat(term.Qualifier.Value, 32)
			return float32(f), err
		}
	}
	return 0, fmt.Errorf("no qualifier")
}

func (q *Query) QualifierString(key string) (string, error) {
	if q == nil {
		return "", fmt.Errorf("nil query")
	}

	if len(q.Terms) == 0 {
		return "", fmt.Errorf("empty query")
	}

	for _, term := range q.Terms {
		if term.Qualifier != nil && term.Qualifier.Key == key {
			return term.Qualifier.Value, nil
		}
	}
	return "", fmt.Errorf("no qualifier")
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
