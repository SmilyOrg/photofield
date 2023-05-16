package search

import (
	"fmt"
	"strings"
)

type Where struct {
	SQL   string
	Texts []string
}

func (w Where) IsEmpty() bool {
	return w.SQL == ""
}

func (w Where) String() string {
	return fmt.Sprintf("where %s [%s]", w.SQL, strings.Join(w.Texts, ", "))
}

func (q *Query) Where(key string, col string) Where {
	w := Where{}
	if q == nil {
		return w
	}
	for _, term := range q.Terms {
		if !w.IsEmpty() {
			w.SQL += " AND "
		}
		tw := term.Where(key, col)
		w.SQL += tw.SQL
		w.Texts = append(w.Texts, tw.Texts...)
	}
	return w
}

func (t *Term) Where(key string, col string) Where {
	if t.Qualifier != nil && t.Qualifier.Key == key {
		return Where{
			SQL:   fmt.Sprintf("%s = ?", col),
			Texts: []string{t.Qualifier.Value},
		}
	}
	return Where{}
}
