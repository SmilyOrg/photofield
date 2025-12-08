package search

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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
		a, err = parseFlexibleDate(value[2:], true)
		if err != nil {
			return
		}
		return a, time.Time{}, nil
	}
	if strings.HasPrefix(value, "<=") {
		b, err = parseFlexibleDate(value[2:], false)
		if err != nil {
			return
		}
		return time.Time{}, b, nil
	}
	if strings.HasPrefix(value, ">") {
		a, err = parseFlexibleDate(value[1:], true)
		if err != nil {
			return
		}
		a = a.AddDate(0, 0, 1)
		return a, time.Time{}, nil
	}
	if strings.HasPrefix(value, "<") {
		b, err = parseFlexibleDate(value[1:], false)
		if err != nil {
			return
		}
		b = b.AddDate(0, 0, -1)
		return time.Time{}, b, nil
	}

	// Check for range format
	dateRange := strings.SplitN(value, "..", 2)
	if len(dateRange) == 2 {
		a, err = parseFlexibleDate(dateRange[0], true)
		if err != nil {
			err = fmt.Errorf("failed to parse start date: %v", err)
			return
		}

		b, err = parseFlexibleDate(dateRange[1], false)
		if err != nil {
			err = fmt.Errorf("failed to parse end date: %v", err)
			return
		}
		return
	}

	// Single date/format
	a, err = parseFlexibleDate(value, true)
	if err != nil {
		return
	}
	b, err = parseFlexibleDate(value, false)
	if err != nil {
		return
	}

	return
}

// parseFlexibleDate parses various date formats and wildcards
// isStart determines how to interpret partial dates (start or end of period)
func parseFlexibleDate(value string, isStart bool) (date time.Time, err error) {
	// Validate basic format before processing
	if err := validateDateFormat(value); err != nil {
		return time.Time{}, err
	}

	// Try different date formats
	parts := strings.SplitN(value, "-", 3)
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid date format")
	}
	switch len(parts) {
	case 1:
		// Year-only: YYYY
		year, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, err
		}
		d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		if !isStart {
			d = d.AddDate(1, 0, 0)
		}
		return d, nil

	case 2:
		// Year-month: YYYY-MM
		year, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, err
		}
		month, err := parseMonth(parts[1], isStart)
		if err != nil {
			return time.Time{}, err
		}
		d := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		if !isStart {
			d = d.AddDate(0, 1, 0)
		}
		return d, nil

	case 3:
		// Year-month-day: YYYY-MM-DD
		year, err := parseYear(parts[0], isStart)
		if err != nil {
			return time.Time{}, err
		}
		month, err := parseMonth(parts[1], isStart)
		if err != nil {
			return time.Time{}, err
		}
		day, err := parseDay(parts[2], isStart)
		if err != nil {
			return time.Time{}, err
		}
		day = normalizeDay(year, month, day)
		d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if !isStart {
			d = d.AddDate(0, 0, 1)
		}
		return d, nil
	}

	return time.Time{}, fmt.Errorf("invalid date format")
}

// parseYear parses a wildcard year format
func parseYear(value string, isStart bool) (int, error) {
	if value == "*" {
		if isStart {
			return 1, nil // Start at year 1
		} else {
			return 9999, nil // End at year 9999
		}
	}
	year, err := strconv.Atoi(value)
	if err != nil || year < 1000 || year > 9999 {
		return 0, fmt.Errorf("invalid year format")
	}
	return year, nil
}

// parseMonth parses a wildcard month format
func parseMonth(value string, isStart bool) (int, error) {
	if value == "*" {
		if isStart {
			return 1, nil // Start at January
		} else {
			return 12, nil // End at December
		}
	}
	month, err := strconv.Atoi(value)
	if err != nil || month < 1 || month > 12 {
		return 0, fmt.Errorf("invalid month format")
	}
	return month, nil
}

// parseDay parses a wildcard day format
func parseDay(value string, isStart bool) (int, error) {
	if value == "*" {
		if isStart {
			return 1, nil // Start at first day of month
		} else {
			return 31, nil // End at last day of month
		}
	}
	day, err := strconv.Atoi(value)
	if err != nil || day < 1 || day > 31 {
		return 0, fmt.Errorf("invalid day format")
	}
	return day, nil
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
			if err != nil || year < 1000 || year > 9999 {
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
			if err != nil || year < 1000 || year > 9999 {
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
			if err != nil || (year < 1000 && len(parts[0]) > 1) || year > 9999 {
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
