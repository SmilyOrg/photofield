package search

import (
	"fmt"
	"os"

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

var ErrNilQuery = fmt.Errorf("nil query")
var ErrNotFound = fmt.Errorf("not found")

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

func (q *Query) QualifierTerms(key string) []*Term {
	if q == nil {
		return nil
	}
	var terms []*Term
	for _, term := range q.Terms {
		if term.Qualifier != nil && term.Qualifier.Key == key {
			terms = append(terms, term)
		}
	}
	return terms
}

func (q *Query) HasQualifiers() bool {
	if q == nil {
		return false
	}
	for _, term := range q.Terms {
		if term.Qualifier != nil {
			return true
		}
	}
	return false
}

func (q *Query) Words() string {
	if q == nil {
		return ""
	}
	var words string
	for _, term := range q.Terms {
		if term.Word != nil {
			words += *term.Word + " "
		}
	}
	if len(words) == 0 {
		return ""
	}
	return words[:len(words)-1]
}
