package layout

import (
	"photofield/internal/render"
	"testing"
	"time"
)

// TestShuffleConstantsMatchRender verifies that layout.Order shuffle constants
// match the corresponding render package constants
func TestShuffleConstantsMatchRender(t *testing.T) {
	tests := []struct {
		name        string
		layoutConst Order
		renderConst int
	}{
		{"ShuffleHourly", ShuffleHourly, render.ShuffleHourly},
		{"ShuffleDaily", ShuffleDaily, render.ShuffleDaily},
		{"ShuffleWeekly", ShuffleWeekly, render.ShuffleWeekly},
		{"ShuffleMonthly", ShuffleMonthly, render.ShuffleMonthly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.layoutConst) != tt.renderConst {
				t.Errorf("%s mismatch: layout=%d, render=%d",
					tt.name, int(tt.layoutConst), tt.renderConst)
			}
		})
	}
}

func TestComputeShuffleSeed(t *testing.T) {
	// Use a fixed location for consistent test results
	loc := time.UTC

	tests := []struct {
		name     string
		order    Order
		time     time.Time
		expected int64
	}{
		// Hourly tests
		{
			name:     "hourly - middle of hour",
			order:    ShuffleHourly,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Date(2024, 6, 15, 14, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "hourly - start of hour",
			order:    ShuffleHourly,
			time:     time.Date(2024, 6, 15, 14, 0, 0, 0, loc),
			expected: time.Date(2024, 6, 15, 14, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "hourly - end of hour",
			order:    ShuffleHourly,
			time:     time.Date(2024, 6, 15, 14, 59, 59, 0, loc),
			expected: time.Date(2024, 6, 15, 14, 0, 0, 0, loc).Unix(),
		},

		// Daily tests
		{
			name:     "daily - middle of day",
			order:    ShuffleDaily,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Date(2024, 6, 15, 0, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "daily - start of day",
			order:    ShuffleDaily,
			time:     time.Date(2024, 6, 15, 0, 0, 0, 0, loc),
			expected: time.Date(2024, 6, 15, 0, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "daily - end of day",
			order:    ShuffleDaily,
			time:     time.Date(2024, 6, 15, 23, 59, 59, 0, loc),
			expected: time.Date(2024, 6, 15, 0, 0, 0, 0, loc).Unix(),
		},

		// Weekly tests - Monday is the start of week
		{
			name:     "weekly - Monday",
			order:    ShuffleWeekly,
			time:     time.Date(2024, 6, 17, 14, 30, 0, 0, loc), // Monday
			expected: time.Date(2024, 6, 17, 0, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "weekly - Tuesday",
			order:    ShuffleWeekly,
			time:     time.Date(2024, 6, 18, 14, 30, 0, 0, loc),      // Tuesday
			expected: time.Date(2024, 6, 17, 0, 0, 0, 0, loc).Unix(), // Previous Monday
		},
		{
			name:     "weekly - Sunday",
			order:    ShuffleWeekly,
			time:     time.Date(2024, 6, 16, 14, 30, 0, 0, loc),      // Sunday
			expected: time.Date(2024, 6, 10, 0, 0, 0, 0, loc).Unix(), // Previous Monday
		},
		{
			name:     "weekly - Saturday",
			order:    ShuffleWeekly,
			time:     time.Date(2024, 6, 22, 14, 30, 0, 0, loc),      // Saturday
			expected: time.Date(2024, 6, 17, 0, 0, 0, 0, loc).Unix(), // Previous Monday
		},

		// Monthly tests
		{
			name:     "monthly - middle of month",
			order:    ShuffleMonthly,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Date(2024, 6, 1, 0, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "monthly - first of month",
			order:    ShuffleMonthly,
			time:     time.Date(2024, 6, 1, 0, 0, 0, 0, loc),
			expected: time.Date(2024, 6, 1, 0, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "monthly - last day of month",
			order:    ShuffleMonthly,
			time:     time.Date(2024, 6, 30, 23, 59, 59, 0, loc),
			expected: time.Date(2024, 6, 1, 0, 0, 0, 0, loc).Unix(),
		},
		{
			name:     "monthly - February (leap year)",
			order:    ShuffleMonthly,
			time:     time.Date(2024, 2, 29, 12, 0, 0, 0, loc),
			expected: time.Date(2024, 2, 1, 0, 0, 0, 0, loc).Unix(),
		},

		// None/invalid order
		{
			name:     "none order",
			order:    None,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: 0,
		},
		{
			name:     "date asc order",
			order:    DateAsc,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeShuffleSeed(tt.order, tt.time)
			if result != tt.expected {
				t.Errorf("ComputeShuffleSeed(%v, %v) = %d; want %d",
					tt.order, tt.time, result, tt.expected)
			}
		})
	}
}

func TestComputeShuffleSeed_Consistency(t *testing.T) {
	loc := time.UTC

	t.Run("same hour produces same seed", func(t *testing.T) {
		time1 := time.Date(2024, 6, 15, 14, 10, 0, 0, loc)
		time2 := time.Date(2024, 6, 15, 14, 50, 0, 0, loc)

		seed1 := ComputeShuffleSeed(ShuffleHourly, time1)
		seed2 := ComputeShuffleSeed(ShuffleHourly, time2)

		if seed1 != seed2 {
			t.Errorf("Expected same seed for same hour, got %d and %d", seed1, seed2)
		}
	})

	t.Run("different hours produce different seeds", func(t *testing.T) {
		time1 := time.Date(2024, 6, 15, 14, 30, 0, 0, loc)
		time2 := time.Date(2024, 6, 15, 15, 30, 0, 0, loc)

		seed1 := ComputeShuffleSeed(ShuffleHourly, time1)
		seed2 := ComputeShuffleSeed(ShuffleHourly, time2)

		if seed1 == seed2 {
			t.Errorf("Expected different seeds for different hours, both got %d", seed1)
		}
	})

	t.Run("same day produces same seed", func(t *testing.T) {
		time1 := time.Date(2024, 6, 15, 8, 0, 0, 0, loc)
		time2 := time.Date(2024, 6, 15, 20, 0, 0, 0, loc)

		seed1 := ComputeShuffleSeed(ShuffleDaily, time1)
		seed2 := ComputeShuffleSeed(ShuffleDaily, time2)

		if seed1 != seed2 {
			t.Errorf("Expected same seed for same day, got %d and %d", seed1, seed2)
		}
	})

	t.Run("different days produce different seeds", func(t *testing.T) {
		time1 := time.Date(2024, 6, 15, 12, 0, 0, 0, loc)
		time2 := time.Date(2024, 6, 16, 12, 0, 0, 0, loc)

		seed1 := ComputeShuffleSeed(ShuffleDaily, time1)
		seed2 := ComputeShuffleSeed(ShuffleDaily, time2)

		if seed1 == seed2 {
			t.Errorf("Expected different seeds for different days, both got %d", seed1)
		}
	})

	t.Run("same week produces same seed", func(t *testing.T) {
		// Monday and Friday of same week
		time1 := time.Date(2024, 6, 17, 10, 0, 0, 0, loc) // Monday
		time2 := time.Date(2024, 6, 21, 15, 0, 0, 0, loc) // Friday

		seed1 := ComputeShuffleSeed(ShuffleWeekly, time1)
		seed2 := ComputeShuffleSeed(ShuffleWeekly, time2)

		if seed1 != seed2 {
			t.Errorf("Expected same seed for same week, got %d and %d", seed1, seed2)
		}
	})

	t.Run("different weeks produce different seeds", func(t *testing.T) {
		time1 := time.Date(2024, 6, 17, 12, 0, 0, 0, loc) // Monday week 1
		time2 := time.Date(2024, 6, 24, 12, 0, 0, 0, loc) // Monday week 2

		seed1 := ComputeShuffleSeed(ShuffleWeekly, time1)
		seed2 := ComputeShuffleSeed(ShuffleWeekly, time2)

		if seed1 == seed2 {
			t.Errorf("Expected different seeds for different weeks, both got %d", seed1)
		}
	})

	t.Run("same month produces same seed", func(t *testing.T) {
		time1 := time.Date(2024, 6, 1, 0, 0, 0, 0, loc)
		time2 := time.Date(2024, 6, 30, 23, 59, 59, 0, loc)

		seed1 := ComputeShuffleSeed(ShuffleMonthly, time1)
		seed2 := ComputeShuffleSeed(ShuffleMonthly, time2)

		if seed1 != seed2 {
			t.Errorf("Expected same seed for same month, got %d and %d", seed1, seed2)
		}
	})

	t.Run("different months produce different seeds", func(t *testing.T) {
		time1 := time.Date(2024, 6, 15, 12, 0, 0, 0, loc)
		time2 := time.Date(2024, 7, 15, 12, 0, 0, 0, loc)

		seed1 := ComputeShuffleSeed(ShuffleMonthly, time1)
		seed2 := ComputeShuffleSeed(ShuffleMonthly, time2)

		if seed1 == seed2 {
			t.Errorf("Expected different seeds for different months, both got %d", seed1)
		}
	})
}

func TestComputeShuffleSeed_WeekBoundaries(t *testing.T) {
	loc := time.UTC

	// Test that Sunday rolls back to previous Monday
	sunday := time.Date(2024, 6, 23, 12, 0, 0, 0, loc) // Sunday
	monday := time.Date(2024, 6, 17, 0, 0, 0, 0, loc)  // Expected Monday (6 days earlier)

	result := ComputeShuffleSeed(ShuffleWeekly, sunday)
	expected := monday.Unix()

	if result != expected {
		resultTime := time.Unix(result, 0).UTC()
		t.Errorf("Sunday should roll back to previous Monday.\nGot: %v (%d)\nExpected: %v (%d)",
			resultTime, result, monday, expected)
	}

	// Test all days of a week map to the same Monday
	weekStart := time.Date(2024, 6, 17, 0, 0, 0, 0, loc) // Monday
	expectedSeed := weekStart.Unix()

	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		seed := ComputeShuffleSeed(ShuffleWeekly, day)
		if seed != expectedSeed {
			t.Errorf("%s (%v) produced seed %d, expected %d",
				days[i], day, seed, expectedSeed)
		}
	}
}
