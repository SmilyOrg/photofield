package shuffle

import (
	"testing"
	"time"
)

func TestTruncateTime(t *testing.T) {
	loc := time.UTC

	tests := []struct {
		name     string
		order    Order
		time     time.Time
		expected time.Time
	}{
		// Hourly tests
		{
			name:     "hourly - middle of hour",
			order:    Hourly,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Date(2024, 6, 15, 14, 0, 0, 0, loc),
		},
		{
			name:     "hourly - start of hour",
			order:    Hourly,
			time:     time.Date(2024, 6, 15, 14, 0, 0, 0, loc),
			expected: time.Date(2024, 6, 15, 14, 0, 0, 0, loc),
		},

		// Daily tests
		{
			name:     "daily - middle of day",
			order:    Daily,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Date(2024, 6, 15, 0, 0, 0, 0, loc),
		},
		{
			name:     "daily - end of day",
			order:    Daily,
			time:     time.Date(2024, 6, 15, 23, 59, 59, 0, loc),
			expected: time.Date(2024, 6, 15, 0, 0, 0, 0, loc),
		},

		// Weekly tests
		{
			name:     "weekly - Monday",
			order:    Weekly,
			time:     time.Date(2024, 6, 17, 14, 30, 0, 0, loc),
			expected: time.Date(2024, 6, 17, 0, 0, 0, 0, loc),
		},
		{
			name:     "weekly - Sunday",
			order:    Weekly,
			time:     time.Date(2024, 6, 16, 14, 30, 0, 0, loc),
			expected: time.Date(2024, 6, 10, 0, 0, 0, 0, loc), // Previous Monday
		},
		{
			name:     "weekly - Saturday",
			order:    Weekly,
			time:     time.Date(2024, 6, 22, 14, 30, 0, 0, loc),
			expected: time.Date(2024, 6, 17, 0, 0, 0, 0, loc), // Previous Monday
		},

		// Monthly tests
		{
			name:     "monthly - middle of month",
			order:    Monthly,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Date(2024, 6, 1, 0, 0, 0, 0, loc),
		},
		{
			name:     "monthly - last day",
			order:    Monthly,
			time:     time.Date(2024, 6, 30, 23, 59, 59, 0, loc),
			expected: time.Date(2024, 6, 1, 0, 0, 0, 0, loc),
		},

		// Invalid order
		{
			name:     "invalid order",
			order:    999,
			time:     time.Date(2024, 6, 15, 14, 30, 45, 0, loc),
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateTime(tt.order, tt.time)
			if !result.Equal(tt.expected) {
				t.Errorf("TruncateTime(%d, %v) = %v; want %v",
					tt.order, tt.time, result, tt.expected)
			}
		})
	}
}

func TestTruncateTime_WeekBoundaries(t *testing.T) {
	loc := time.UTC

	// Test all days of a week map to the same Monday
	weekStart := time.Date(2024, 6, 17, 0, 0, 0, 0, loc) // Monday
	expectedTime := weekStart

	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for i := 0; i < 7; i++ {
		day := weekStart.AddDate(0, 0, i)
		result := TruncateTime(Weekly, day)
		if !result.Equal(expectedTime) {
			t.Errorf("%s (%v) produced %v, expected %v",
				days[i], day, result, expectedTime)
		}
	}
}

func TestTruncateTime_Consistency(t *testing.T) {
	// Test that calling TruncateTime multiple times returns the same result
	orders := []struct {
		name  string
		order Order
	}{
		{"hourly", Hourly},
		{"daily", Daily},
		{"weekly", Weekly},
		{"monthly", Monthly},
	}

	for _, tt := range orders {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()

			first := TruncateTime(tt.order, now)
			time.Sleep(10 * time.Millisecond)
			second := TruncateTime(tt.order, now)

			if !first.Equal(second) {
				t.Errorf("TruncateTime(%d) not consistent: %v != %v",
					tt.order, first, second)
			}
		})
	}
}
