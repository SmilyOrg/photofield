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
	Year  bool `json:"year"`
	Month bool `json:"month"`
	Day   bool `json:"day"`
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
	FieldMeta    `json:"meta"`
	From         time.Time    `json:"from"`
	To           time.Time    `json:"to"`
	FromWildcard DateWildcard `json:"from_wildcard"`
	ToWildcard   DateWildcard `json:"to_wildcard"`
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
	r.Name = key

	if q == nil {
		r.Error = ErrNilQuery
		return
	}

	terms := q.QualifierTerms(key)
	if len(terms) == 0 {
		r.Error = ErrNotFound
		return
	}

	if len(terms) > 1 {
		r.Error = fmt.Errorf("multiple qualifiers %s", key)
		return
	}

	term := terms[0]
	value := term.Qualifier.Value
	r.Present = true
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

// QualifierDateRange extracts and parses a date range from a query qualifier.
// It supports various date formats including comparison operators, ranges, wildcards, and partial dates.
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

	// Check for comparison operators
	if strings.HasPrefix(value, ">=") {
		a, _, err = parseFlexibleDate(value[2:], true)
		if err != nil {
			return
		}
		return a, time.Time{}, nil
	}
	if strings.HasPrefix(value, "<=") {
		b, _, err = parseFlexibleDate(value[2:], false)
		if err != nil {
			return
		}
		return time.Time{}, b, nil
	}
	if strings.HasPrefix(value, ">") {
		a, _, err = parseFlexibleDate(value[1:], true)
		if err != nil {
			return
		}
		a = a.AddDate(0, 0, 1)
		return a, time.Time{}, nil
	}
	if strings.HasPrefix(value, "<") {
		b, _, err = parseFlexibleDate(value[1:], false)
		if err != nil {
			return
		}
		b = b.AddDate(0, 0, -1)
		return time.Time{}, b, nil
	}

	// Check for range format
	dateRange := strings.SplitN(value, "..", 2)
	if len(dateRange) == 2 {
		a, _, err = parseFlexibleDate(dateRange[0], true)
		if err != nil {
			err = fmt.Errorf("failed to parse start date: %v", err)
			return
		}

		b, _, err = parseFlexibleDate(dateRange[1], false)
		if err != nil {
			err = fmt.Errorf("failed to parse end date: %v", err)
			return
		}
		return
	}

	// Single date/format
	a, _, err = parseFlexibleDate(value, true)
	if err != nil {
		return
	}
	b, _, err = parseFlexibleDate(value, false)
	if err != nil {
		return
	}

	return
}

// parseFlexibleDate parses various date formats and wildcards
// isStart determines how to interpret partial dates (start or end of period)
func parseFlexibleDate(value string, isStart bool) (date time.Time, w DateWildcard, err error) {
	// Validate basic format before processing
	if err := validateDateFormat(value); err != nil {
		return time.Time{}, DateWildcard{}, err
	}

	// Try different date formats
	parts := strings.SplitN(value, "-", 3)
	if len(parts) == 0 {
		return time.Time{}, DateWildcard{}, fmt.Errorf("invalid date format")
	}
	switch len(parts) {
	case 1:
		// Year-only: YYYY
		year, yearWildcard, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, DateWildcard{}, err
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

// parseYear parses a wildcard year format
func parseYear(value string, isStart bool) (int, bool, error) {
	if value == "*" {
		if isStart {
			return 1, true, nil // Start at year 1
		} else {
			return MAX_YEAR, true, nil // End at year MAX_YEAR
		}
	}
	year, err := strconv.Atoi(value)
	if err != nil || year < MIN_YEAR || year > MAX_YEAR {
		return 0, false, fmt.Errorf("invalid year format")
	}
	return year, false, nil
}

// parseMonth parses a wildcard month format
func parseMonth(value string, isStart bool) (int, bool, error) {
	if value == "*" {
		if isStart {
			return 1, true, nil // Start at January
		} else {
			return 12, true, nil // End at December
		}
	}
	month, err := strconv.Atoi(value)
	if err != nil || month < 1 || month > 12 {
		return 0, false, fmt.Errorf("invalid month format")
	}
	return month, false, nil
}

// parseDay parses a wildcard day format
func parseDay(value string, isStart bool) (int, bool, error) {
	if value == "*" {
		if isStart {
			return 1, true, nil // Start at first day of month
		} else {
			return 31, true, nil // End at last day of month (will be normalized)
		}
	}
	day, err := strconv.Atoi(value)
	if err != nil || day < 1 || day > 31 {
		return 0, false, fmt.Errorf("invalid day format")
	}
	return day, false, nil
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

// validateDateFormat performs basic validation on date format
func validateDateFormat(value string) error {
	// Check for invalid separators
	if strings.Contains(value, "/") {
		return fmt.Errorf("invalid date separator, use '-'")
	}

	// Check if it's a valid format pattern
	if !strings.Contains(value, "-") && !strings.Contains(value, "*") {
		// Could be a year-only format
		if len(value) == 4 {
			year, err := strconv.Atoi(value)
			if err != nil || year < MIN_YEAR || year > MAX_YEAR {
				return fmt.Errorf("invalid year format")
			}
			return nil
		}
		if len(value) == 8 {
			// Possibly unseparated date like 20251210
			return fmt.Errorf("invalid date format, use '-' separators")
		}
		if len(value) == 2 {
			// Two-digit year like 99-01-01
			return fmt.Errorf("invalid year format")
		}
		// Any other format without separators
		return fmt.Errorf("invalid date format")
	}

	// Validate parts
	parts := strings.Split(value, "-")
	if len(parts) == 2 {
		// Year-Month format
		if parts[0] != "*" {
			year, err := strconv.Atoi(parts[0])
			if err != nil || year < MIN_YEAR || year > MAX_YEAR {
				return fmt.Errorf("invalid year")
			}
		}
		if parts[1] != "*" {
			month, err := strconv.Atoi(parts[1])
			if err != nil || month < 1 || month > 12 {
				return fmt.Errorf("invalid month")
			}
		}
	} else if len(parts) == 3 {
		// Full date format
		if parts[0] != "*" {
			year, err := strconv.Atoi(parts[0])
			if err != nil || (year < MIN_YEAR && len(parts[0]) > 1) || year > MAX_YEAR {
				return fmt.Errorf("invalid year")
			}
		}
		if parts[1] != "*" {
			month, err := strconv.Atoi(parts[1])
			if err != nil || month < 1 || month > 12 {
				return fmt.Errorf("invalid month")
			}
		}
		if parts[2] != "*" {
			day, err := strconv.Atoi(parts[2])
			if err != nil || day < 1 || day > 31 {
				return fmt.Errorf("invalid day")
			}
		}
	}

	return nil
}
