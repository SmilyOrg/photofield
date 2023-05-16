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

func TestWhereEmpty(t *testing.T) {
	query, err := Parse("")
	if err != nil {
		t.Error(err)
	}
	actual := query.Where("tag", "name")
	expected := Where{
		SQL:   "",
		Texts: nil,
	}
	assert.Equal(t, expected, actual)
}

func TestWhereTag(t *testing.T) {
	query, err := Parse("tag:hello")
	if err != nil {
		t.Error(err)
	}
	actual := query.Where("tag", "name")
	expected := Where{
		SQL:   "name = ?",
		Texts: []string{"hello"},
	}
	assert.Equal(t, expected, actual)
}

func TestWhereTags(t *testing.T) {
	query, err := Parse("tag:hello tag:world")
	if err != nil {
		t.Error(err)
	}
	actual := query.Where("tag", "name")
	expected := Where{
		SQL:   "name = ? AND name = ?",
		Texts: []string{"hello", "world"},
	}
	assert.Equal(t, expected, actual)
}

func TestWhereExifTag(t *testing.T) {
	query, err := Parse("tag:exif:model:dji-osmo-action")
	if err != nil {
		t.Error(err)
	}
	actual := query.Where("tag", "name")
	expected := Where{
		SQL:   "name = ?",
		Texts: []string{"exif:model:dji-osmo-action"},
	}
	assert.Equal(t, expected, actual)
}
