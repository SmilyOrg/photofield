package shuffle

import "time"

// Order represents a shuffle interval type
type Order int

// Order constants for shuffle intervals
// These match the layout.Order enum values
const (
	Hourly  Order = 3
	Daily   Order = 4
	Weekly  Order = 5
	Monthly Order = 6
)

// TruncateTime truncates the given time based on the shuffle order type.
// Returns the truncated time for hourly, daily, weekly, or monthly intervals.
//
// The order parameter should be one of the shuffle order constants (Hourly, Daily, Weekly, Monthly).
// For invalid order values, returns zero time.
func TruncateTime(order Order, t time.Time) time.Time {
	switch order {
	case Hourly:
		return t.Truncate(time.Hour)
	case Daily:
		y, m, day := t.Date()
		loc := t.Location()
		return time.Date(y, m, day, 0, 0, 0, 0, loc)
	case Weekly:
		// Truncate to Monday at midnight local time
		y, m, day := t.Date()
		loc := t.Location()
		weekday := t.Weekday()
		daysFromMonday := int(weekday) - 1
		if daysFromMonday < 0 {
			daysFromMonday = 6 // Sunday
		}
		mondayDate := time.Date(y, m, day-daysFromMonday, 0, 0, 0, 0, loc)
		return mondayDate
	case Monthly:
		y, m, _ := t.Date()
		loc := t.Location()
		return time.Date(y, m, 1, 0, 0, 0, 0, loc)
	default:
		return time.Time{}
	}
}
