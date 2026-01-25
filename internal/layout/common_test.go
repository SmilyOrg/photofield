package layout

import (
	"photofield/internal/layout/shuffle"
	"testing"
)

// TestShuffleConstantsMatchRender verifies that layout.Order shuffle constants
// match the corresponding shuffle package constants
func TestShuffleConstantsMatchRender(t *testing.T) {
	tests := []struct {
		name         string
		layoutConst  Order
		shuffleConst shuffle.Order
	}{
		{"ShuffleHourly", ShuffleHourly, shuffle.Hourly},
		{"ShuffleDaily", ShuffleDaily, shuffle.Daily},
		{"ShuffleWeekly", ShuffleWeekly, shuffle.Weekly},
		{"ShuffleMonthly", ShuffleMonthly, shuffle.Monthly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.layoutConst) != int(tt.shuffleConst) {
				t.Errorf("%s mismatch: layout=%d, shuffle=%d",
					tt.name, int(tt.layoutConst), int(tt.shuffleConst))
			}
		})
	}
}
