package search

import (
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
)

func TestDateRanges(t *testing.T) {
	date := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	tests := []struct {
		input      string
		start, end time.Time
		wantErr    bool
	}{
		{"created:2022-01-01..2022-12-31", date("2022-01-01"), date("2023-01-01"), false},
		{"created:2020-06-15..2020-06-15", date("2020-06-15"), date("2020-06-16"), false},
		{"created:2019-01-01..2019-01-31", date("2019-01-01"), date("2019-02-01"), false},
		{"created:2021-12-01..2022-01-15", date("2021-12-01"), date("2022-01-16"), false},
		{"hello world", time.Time{}, time.Time{}, true},
		{"created:2022-01-01..2022-12-31 created:2023-01-01..2023-12-31", time.Time{}, time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			query, err := Parse(tt.input)
			assert.NoError(t, err)
			start, end, err := query.QualifierDateRange("created")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.start, start)
				assert.Equal(t, tt.end, end)
			}
		})
	}
}

func TestDateRangeFormats(t *testing.T) {
	date := func(s string) time.Time { d, _ := time.Parse("2006-01-02", s); return d }
	tests := []struct {
		input      string
		start, end time.Time
		wantErr    bool
	}{
		// Basic range format
		{"created:2022-01-01..2022-12-31", date("2022-01-01"), date("2023-01-01"), false},
		{"created:2020-06-15..2020-06-15", date("2020-06-15"), date("2020-06-16"), false},
		{"created:2019-01-01..2019-01-31", date("2019-01-01"), date("2019-02-01"), false},
		{"created:2021-12-01..2022-01-15", date("2021-12-01"), date("2022-01-16"), false},
		// No qualifier
		{"hello world", time.Time{}, time.Time{}, true},
		// Multiple qualifiers
		{"created:2022-01-01..2022-12-31 created:2023-01-01..2023-12-31", time.Time{}, time.Time{}, true},
		// Comparison operators
		{"created:>=2025-12-10", date("2025-12-10"), time.Time{}, false},
		{"created:>2025-12-10", date("2025-12-11"), time.Time{}, false},
		{"created:<=2025-12-10", time.Time{}, date("2025-12-11"), false},
		{"created:<2025-12-10", time.Time{}, date("2025-12-10"), false},
		// Exact date
		{"created:2025-12-10", date("2025-12-10"), date("2025-12-11"), false},
		{"created:2024-02-29", date("2024-02-29"), date("2024-03-01"), false},
		{"created:2023-12-31", date("2023-12-31"), date("2024-01-01"), false},
		// Wildcards - any day of specific month
		{"created:*-12-10", date("0001-12-10"), date("9000-12-11"), false},
		{"created:*-01-01", date("0001-01-01"), date("9000-01-02"), false},
		{"created:*-02-29", date("0001-02-28"), date("9000-03-01"), false},
		{"created:*-01-01", date("0001-01-01"), date("9000-01-02"), false},
		{"created:*-12-31", date("0001-12-31"), date("9001-01-01"), false},
		// Wildcards - specific day across all months in year
		{"created:2025-*-10", date("2025-01-10"), date("2025-12-11"), false},
		{"created:2025-*-01", date("2025-01-01"), date("2025-12-02"), false},
		{"created:2025-*-31", date("2025-01-31"), date("2026-01-01"), false},
		// Wildcards - all days in specific month
		{"created:2025-12-*", date("2025-12-01"), date("2026-01-01"), false},
		{"created:2025-01-*", date("2025-01-01"), date("2025-02-01"), false},
		{"created:2024-02-*", date("2024-02-01"), date("2024-03-01"), false},
		{"created:2023-02-*", date("2023-02-01"), date("2023-03-01"), false},
		// Month precision
		{"created:2025-12", date("2025-12-01"), date("2026-01-01"), false},
		{"created:2025-01", date("2025-01-01"), date("2025-02-01"), false},
		{"created:2024-02", date("2024-02-01"), date("2024-03-01"), false},
		// Year precision
		{"created:2025", date("2025-01-01"), date("2026-01-01"), false},
		{"created:2024", date("2024-01-01"), date("2025-01-01"), false},
		{"created:2000", date("2000-01-01"), date("2001-01-01"), false},
		// Year ranges
		{"created:2023..2025", date("2023-01-01"), date("2026-01-01"), false},
		{"created:2020..2020", date("2020-01-01"), date("2021-01-01"), false},
		{"created:2000..2024", date("2000-01-01"), date("2025-01-01"), false},
		// Month ranges
		{"created:2023-03..2025-05", date("2023-03-01"), date("2025-06-01"), false},
		{"created:2025-01..2025-12", date("2025-01-01"), date("2026-01-01"), false},
		{"created:2024-02..2024-02", date("2024-02-01"), date("2024-03-01"), false},
		// Edge cases - invalid day normalization
		{"created:2023-02-31", date("2023-02-28"), date("2023-03-01"), false},
		{"created:2024-02-31", date("2024-02-29"), date("2024-03-01"), false},
		{"created:2023-04-31", date("2023-04-30"), date("2023-05-01"), false},
		{"created:2023-06-31", date("2023-06-30"), date("2023-07-01"), false},
		{"created:2023-09-31", date("2023-09-30"), date("2023-10-01"), false},
		{"created:2023-11-31", date("2023-11-30"), date("2023-12-01"), false},
		{"created:>=2023-02-31", date("2023-02-28"), time.Time{}, false},
		{"created:2023-02-30..2023-03-15", date("2023-02-28"), date("2023-03-16"), false},
		// Edge cases - boundary dates
		{"created:>=2025-01-01", date("2025-01-01"), time.Time{}, false},
		{"created:<=2025-12-31", time.Time{}, date("2026-01-01"), false},
		{"created:<2025-01-01", time.Time{}, date("2025-01-01"), false},
		{"created:>2025-12-31", date("2026-01-01"), time.Time{}, false},
		{"created:99-01-01", date("0099-01-01"), date("0099-01-02"), false},
		// Invalid formats
		{"created:invalid", time.Time{}, time.Time{}, true},
		{"created:2025-13-01", time.Time{}, time.Time{}, true},
		{"created:2025-00-01", time.Time{}, time.Time{}, true},
		{"created:2025-01-00", time.Time{}, time.Time{}, true},
		{"created:2025-01-32", time.Time{}, time.Time{}, true},
		{"created:1-01-01", time.Time{}, time.Time{}, true},
		{"created:20251210", time.Time{}, time.Time{}, true},
		{"created:2025/12/10", time.Time{}, time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			query, err := Parse(tt.input)
			assert.NoError(t, err)
			start, end, err := query.QualifierDateRange("created")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.start, start)
			assert.Equal(t, tt.end, end)
		})
	}
}
