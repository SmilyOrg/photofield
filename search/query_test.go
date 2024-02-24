package search

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func str(s string) *string { return &s }

func TestBareHello(t *testing.T) {
	query, err := ParseDebug("hello")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, query.Terms[0].Word, str("hello"))
}

func TestTag(t *testing.T) {
	query, err := Parse("tag:hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(query.Terms) != 1 {
		t.Fatal("Expected 1 term")
	}
	if query.Terms[0].Qualifier == nil {
		t.Fatal("Expected qualifier")
	}
	if query.Terms[0].Qualifier.Key != "tag" {
		t.Fatalf("Expected 'tag', got '%s'", query.Terms[0].Qualifier.Key)
	}
	if query.Terms[0].Qualifier.Value != "hello" {
		t.Fatalf("Expected 'hello', got '%s'", query.Terms[0].Qualifier.Value)
	}
}

func TestQualifierValues(t *testing.T) {
	query, err := Parse("tag:hello word tag:world hi:there")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(
		t,
		[]string{"hello", "world"},
		query.QualifierValues("tag"),
	)
}

func TestNegation(t *testing.T) {
	query, err := Parse("NOT tag:hello")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, true, query.Terms[0].Not)
	assert.Equal(t, []string{"hello"}, query.QualifierValues("tag"))
}

func TestString(t *testing.T) {
	query, err := Parse(`"a photo of a person"`)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "a photo of a person", *query.Terms[0].String)
}
func TestKnn(t *testing.T) {
	query, err := ParseDebug(`tag:me NOT tag:me:not "a photo of a person" NOT "a photo of nothing" k:5 bias:0.01`)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, query.Terms[0].Qualifier.Key, "tag")
	assert.Equal(t, query.Terms[0].Qualifier.Value, "me")
	assert.Equal(t, query.Terms[1].Qualifier.Key, "tag")
	assert.Equal(t, query.Terms[1].Qualifier.Value, "me:not")
	assert.Equal(t, *query.Terms[2].String, "a photo of a person")
	assert.Equal(t, query.Terms[3].Not, true)
	assert.Equal(t, *query.Terms[3].String, "a photo of nothing")
	assert.Equal(t, query.Terms[4].Qualifier.Key, "k")
	assert.Equal(t, query.Terms[4].Qualifier.Value, "5")
	assert.Equal(t, query.Terms[5].Qualifier.Key, "bias")
	assert.Equal(t, query.Terms[5].Qualifier.Value, "0.01")
}

func TestKnn2(t *testing.T) {
	query, err := ParseDebug(`tag:k tag:m NOT tag:k:not 2`)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, query.Terms[0].Qualifier.Key, "tag")
	assert.Equal(t, query.Terms[0].Qualifier.Value, "k")
	assert.Equal(t, query.Terms[1].Qualifier.Key, "tag")
	assert.Equal(t, query.Terms[1].Qualifier.Value, "m")
	assert.Equal(t, query.Terms[2].Not, true)
	assert.Equal(t, query.Terms[2].Qualifier.Key, "tag")
	assert.Equal(t, query.Terms[2].Qualifier.Value, "k:not")
}
