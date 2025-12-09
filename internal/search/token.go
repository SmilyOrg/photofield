package search

type Token struct {
	Type    string `json:"type"`
	Value   string `json:"value"`
	Not     bool   `json:"not,omitempty"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Key     string `json:"key,omitempty"`     // For qualifiers
	QualVal string `json:"qualVal,omitempty"` // For qualifiers
}

func (term *Term) Token() Token {
	token := Token{
		Not:   term.Not,
		Start: term.Pos.Offset,
		End:   term.EndPos.Offset,
	}

	if term.String != nil {
		token.Type = "string"
		token.Value = *term.String
	} else if term.Qualifier != nil {
		token.Type = "qualifier"
		token.Value = term.Qualifier.Key + ":" + term.Qualifier.Value
		token.Key = term.Qualifier.Key
		token.QualVal = term.Qualifier.Value
	} else if term.Word != nil {
		token.Type = "word"
		token.Value = *term.Word
	}

	return token
}

// Tokens returns a list of Tokens representing the query.
func (q *Query) Tokens() []Token {
	if q == nil {
		return nil
	}
	var tokens []Token
	for _, term := range q.Terms {
		token := term.Token()
		tokens = append(tokens, token)
	}
	return tokens
}
