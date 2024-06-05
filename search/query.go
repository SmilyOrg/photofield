package search

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

func (q *Query) QualifierInt(key string) (int, error) {
	if q == nil {
		return 0, ErrNilQuery
	}

	values := q.QualifierValues(key)
	if len(values) == 0 {
		return 0, ErrNotFound
	}

	if len(values) > 1 {
		return 0, fmt.Errorf("multiple qualifiers %s", key)
	}

	return strconv.Atoi(q.Terms[0].Qualifier.Value)
}

func (q *Query) QualifierFloat32(key string) (float32, error) {
	if q == nil {
		return 0, ErrNilQuery
	}

	values := q.QualifierValues(key)
	if len(values) == 0 {
		return 0, ErrNotFound
	}

	if len(values) > 1 {
		return 0, fmt.Errorf("multiple qualifiers %s", key)
	}

	value := values[0]

	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, fmt.Errorf("failed to parse float32: %v", err)
	}

	return float32(f), nil
}

func (q *Query) QualifierDateRange(key string) (a time.Time, b time.Time, err error) {
	if q == nil {
		err = ErrNilQuery
		return
	}

	values := q.QualifierValues(key)
	if len(values) == 0 {
		err = ErrNotFound
		return
	}

	if len(values) > 1 {
		err = fmt.Errorf("multiple qualifiers %s", key)
		return
	}

	value := values[0]

	dateRange := strings.SplitN(value, "..", 2)
	if len(dateRange) != 2 {
		err = fmt.Errorf("invalid date range format")
		return
	}

	a, err = time.Parse("2006-01-02", dateRange[0])
	if err != nil {
		err = fmt.Errorf("failed to parse start date: %v", err)
		return
	}

	b, err = time.Parse("2006-01-02", dateRange[1])
	if err != nil {
		err = fmt.Errorf("failed to parse end date: %v", err)
		return
	}

	b = b.AddDate(0, 0, 1)

	return
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
