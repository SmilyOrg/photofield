package search

import (
	"testing"
	"time"

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
