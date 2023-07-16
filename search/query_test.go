package search

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestBareHello(t *testing.T) {
	query, err := Parse("hello")
	if err != nil {
		t.Error(err)
	}
	if len(query.Terms) != 1 {
		t.Error("Expected 1 term")
	}
	if query.Terms[0].Word == nil {
		t.Error("Expected word")
	}
	if *query.Terms[0].Word != "hello" {
		t.Errorf("Expected 'hello', got '%s'", *query.Terms[0].Word)
	}
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
