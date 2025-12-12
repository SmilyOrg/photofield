package search

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const MIN_YEAR = 10
const MAX_YEAR = 9000

// DateWildcard indicates which components were wildcards ("*") in the parsed value.
type DateWildcard struct {
	Year  bool `json:"year,omitempty"`
	Month bool `json:"month,omitempty"`
	Day   bool `json:"day,omitempty"`
}

func (w DateWildcard) IsZero() bool {
	return !w.Year && !w.Month && !w.Day
}

func (w DateWildcard) Any() bool {
	return w.Year || w.Month || w.Day
}

func (w DateWildcard) All() bool {
	return w.Year && w.Month && w.Day
}

func (w DateWildcard) Apply(reference time.Time, date time.Time) time.Time {
	y, m, d := reference.Date()
	if w.Year {
		y = date.Year()
	}
	if w.Month {
		m = date.Month()
	}
	if w.Day {
		d = date.Day()
	} else {
		// Special case for February 29th on leap years
		if w.Year && m == 2 && d == 28 && date.Day() == 29 {
			d = 29
		}
	}
	return time.Date(y, m, d, 0, 0, 0, 0, reference.Location())
}

func (w DateWildcard) GreaterThanOrEqual(reference time.Time, date time.Time) bool {
	if !w.Year && reference.Year() < date.Year() {
		return false
	}
	if !w.Month && reference.Month() < date.Month() {
		return false
	}
	if !w.Day && reference.Day() < date.Day() {
		return false
	}
	return true
}

func (w DateWildcard) LessThan(reference time.Time, date time.Time) bool {
	if !w.Year && reference.Year() >= date.Year() {
		return false
	}
	if !w.Month && reference.Month() >= date.Month() {
		return false
	}
	if !w.Day && reference.Day() >= date.Day() {
		return false
	}
	return true
}

// DateRange represents a date range for filtering
type DateRange struct {
	FieldMeta    `json:"meta,omitempty"`
	From         time.Time    `json:"from,omitempty"`
	To           time.Time    `json:"to,omitempty"`
	FromWildcard DateWildcard `json:"from_wildcard,omitempty"`
	ToWildcard   DateWildcard `json:"to_wildcard,omitempty"`
}

// IsZero returns true if both From and To are zero
func (r DateRange) IsZero() bool {
	return r.From.IsZero() && r.To.IsZero()
}

func (r DateRange) Match(date time.Time) bool {
	if !r.Present {
		return true
	}
	if r.FromWildcard.Any() && !r.FromWildcard.All() {
		fromDate := r.FromWildcard.Apply(r.From, date)
		if date.Before(fromDate) {
			return false
		}
	}
	if r.ToWildcard.Any() && !r.ToWildcard.All() {
		toDate := r.ToWildcard.Apply(r.To, date)
		if date.After(toDate) {
			return false
		}
	}
	return true
}

func (q *Query) ExpressionDateRange(key string) (r DateRange) {

	if q == nil {
		return
	}

	terms := q.QualifierTerms(key)
	if len(terms) == 0 {
		return
	}

	if len(terms) > 1 {
		r.Error = fmt.Errorf("multiple qualifiers %s", key)
		return
	}

	term := terms[0]
	value := term.Qualifier.Value
	r.Present = true
	r.Name = key
	r.Token = term.Token()

	// Check for comparison operators
	if strings.HasPrefix(value, ">=") {
		r.From, r.FromWildcard, r.Error = parseFlexibleDate(value[2:], true)
		return
	}
	if strings.HasPrefix(value, "<=") {
		r.To, r.ToWildcard, r.Error = parseFlexibleDate(value[2:], false)
		return
	}
	if strings.HasPrefix(value, ">") {
		r.From, r.FromWildcard, r.Error = parseFlexibleDate(value[1:], true)
		if r.Error != nil {
			return
		}
		r.From = r.From.AddDate(0, 0, 1)
		return
	}
	if strings.HasPrefix(value, "<") {
		r.To, r.ToWildcard, r.Error = parseFlexibleDate(value[1:], false)
		if r.Error != nil {
			return
		}
		r.To = r.To.AddDate(0, 0, -1)
		return
	}

	// Check for range format
	dateRange := strings.SplitN(value, "..", 2)
	if len(dateRange) == 2 {
		r.From, r.FromWildcard, r.Error = parseFlexibleDate(dateRange[0], true)
		if r.Error != nil {
			r.Error = fmt.Errorf("failed to parse start date: %w", r.Error)
			return
		}

		r.To, r.ToWildcard, r.Error = parseFlexibleDate(dateRange[1], false)
		if r.Error != nil {
			r.Error = fmt.Errorf("failed to parse end date: %w", r.Error)
			return
		}
		return
	}

	// Single date/format
	r.From, r.FromWildcard, r.Error = parseFlexibleDate(value, true)
	if r.Error != nil {
		return
	}
	r.To, r.ToWildcard, r.Error = parseFlexibleDate(value, false)
	if r.Error != nil {
		return
	}

	return
}

// parseFlexibleDate parses various date formats and wildcards
// isStart determines how to interpret partial dates (start or end of period)
func parseFlexibleDate(value string, isStart bool) (date time.Time, w DateWildcard, err error) {
	// Try different date formats
	parts := strings.SplitN(value, "-", 3)
	if len(parts) == 0 {
		return time.Time{}, DateWildcard{}, fmt.Errorf("invalid date (use YYYY-MM-DD)")
	}
	switch len(parts) {
	case 1:
		// Year-only: YYYY
		year, yearWildcard, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, fmt.Errorf("invalid date (use YYYY-MM-DD): %w", err)
		}
		w.Year = yearWildcard
		d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		if !isStart {
			d = d.AddDate(1, 0, 0)
		}
		return d, w, nil

	case 2:
		// Year-month: YYYY-MM
		year, yearWildcard, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, err
		}
		month, monthWildcard, err := parseMonth(parts[1], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, err
		}
		w.Year = yearWildcard
		w.Month = monthWildcard
		d := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		if !isStart {
			d = d.AddDate(0, 1, 0)
		}
		return d, w, nil

	case 3:
		// Year-month-day: YYYY-MM-DD
		year, yearWildcard, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, err
		}
		month, monthWildcard, err := parseMonth(parts[1], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, err
		}
		day, dayWildcard, err := parseDay(parts[2], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, err
		}
		w.Year = yearWildcard
		w.Month = monthWildcard
		w.Day = dayWildcard
		day = normalizeDay(year, month, day)
		d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if !isStart {
			d = d.AddDate(0, 0, 1)
		}
		return d, w, nil
	}

	return time.Time{}, DateWildcard{}, fmt.Errorf("invalid date format")
}

// parseInRange parses a value that can be a number or a wildcard "*"
func parseInRange(name, value string, min, max int, isStart bool) (int, bool, error) {
	if value == "*" {
		if isStart {
			return min, true, nil
		} else {
			return max, true, nil
		}
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0, false, fmt.Errorf("%s not a number: %w", name, err)
	}
	if v < min {
		return 0, false, fmt.Errorf("%s below minimum %d", name, min)
	}
	if v > max {
		return 0, false, fmt.Errorf("%s above maximum %d", name, max)
	}
	return v, false, nil
}

// parseYear parses a wildcard year format
func parseYear(value string, isStart bool) (int, bool, error) {
	return parseInRange("year", value, MIN_YEAR, MAX_YEAR, isStart)
}

// parseMonth parses a wildcard month format
func parseMonth(value string, isStart bool) (int, bool, error) {
	return parseInRange("month", value, 1, 12, isStart)
}

// parseDay parses a wildcard day format
func parseDay(value string, isStart bool) (int, bool, error) {
	return parseInRange("day", value, 1, 31, isStart)
}

// normalizeDay adjusts invalid days to the last valid day of the month
func normalizeDay(year, month, day int) int {
	// Get last day of month by going to next month day 0
	lastDay := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		return lastDay
	}
	return day
}
