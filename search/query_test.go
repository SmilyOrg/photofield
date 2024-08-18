package search

import (
	"testing"
	"time"

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

func TestWords(t *testing.T) {
	query, err := Parse("hello   world created:2016-04-29..2016-07-04")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(
		t,
		[]string{"2016-04-29..2016-07-04"},
		query.QualifierValues("created"),
	)
	assert.Equal(t, "hello world", query.Words())
}

func TestEmptyWords(t *testing.T) {
	query, err := Parse("tag:hello")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "", query.Words())
}
func TestQualifierDateRange(t *testing.T) {
	query, err := Parse("created:2022-01-01..2022-12-31")
	if err != nil {
		t.Error(err)
	}

	startDate, endDate, err := query.QualifierDateRange("created")
	if err != nil {
		t.Error(err)
	}

	expectedStartDate, _ := time.Parse("2006-01-02", "2022-01-01")
	expectedEndDate, _ := time.Parse("2006-01-02", "2023-01-01")

	if !startDate.Equal(expectedStartDate) {
		t.Errorf("Expected start date '%s', got '%s'", expectedStartDate.Format("2006-01-02"), startDate.Format("2006-01-02"))
	}

	if !endDate.Equal(expectedEndDate) {
		t.Errorf("Expected end date '%s', got '%s'", expectedEndDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	}
}

func TestQualifierDateRangeNoQualifier(t *testing.T) {
	query, err := Parse("hello world")
	if err != nil {
		t.Error(err)
	}

	_, _, err = query.QualifierDateRange("created")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestQualifierDateRangeMultipleQualifiers(t *testing.T) {
	query, err := Parse("created:2022-01-01..2022-12-31 created:2023-01-01..2023-12-31")
	if err != nil {
		t.Error(err)
	}

	_, _, err = query.QualifierDateRange("created")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestQualifierDateRangeInvalidDateFormat(t *testing.T) {
	query, err := Parse("created:2022-01-01..2022-12-31")
	if err != nil {
		t.Error(err)
	}

	_, _, err = query.QualifierDateRange("invalid")
	if err == nil {
		t.Error("Expected error, got nil")
	}
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
