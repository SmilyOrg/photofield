package collection

import (
	"context"
	"time"

	"github.com/golang/geo/s2"

	"photofield/internal/image"
)

const (
	eventGapTime    = time.Hour
	locationGapTime = 15 * 60 // seconds
	locationGapDist = 1.0     // km
)

// EventSummary represents a time-bounded event within a collection.
type EventSummary struct {
	Index         int        `json:"index,omitempty"`
	CreatedAfter  string     `json:"created_after"`
	CreatedBefore string     `json:"created_before"`
	PhotoCount    int        `json:"photo_count"`
	LocationCount int        `json:"location_count,omitempty"`
	Locations     []string   `json:"locations,omitempty"`
}

// SplitIntoEvents splits a collection's photos into events based on time and
// location proximity. Photos are grouped when they fall within the same day
// and are no more than eventGapTime apart. Reverse-geocoding is applied to
// photo locations that are >1km apart and >15 minutes apart.
func (collection *Collection) SplitIntoEvents(ctx context.Context, source *image.Source) ([]EventSummary, error) {
	infos, _ := collection.GetInfos(source, image.ListOptions{})

	var events []EventSummary
	var current *EventSummary
	var lastPhotoTime time.Time
	var lastLocTime time.Time
	var lastLatLng s2.LatLng
	locations := make(map[string]struct{})

	for info := range infos {
		photoTime := info.DateTime
		if photoTime.IsZero() {
			continue
		}

		if current == nil {
			current = &EventSummary{
				CreatedAfter: photoTime.Format(time.RFC3339),
			}
		} else {
			elapsed := photoTime.Sub(lastPhotoTime)
			if elapsed < 0 {
				elapsed = -elapsed
			}
			if elapsed > eventGapTime || !sameDay(lastPhotoTime, photoTime) {
				// Finalize previous event
				current.CreatedBefore = lastPhotoTime.Format(time.RFC3339)
				current.Locations = make([]string, 0, len(locations))
				for loc := range locations {
					current.Locations = append(current.Locations, loc)
				}
				events = append(events, *current)

				// Start new event
				current = &EventSummary{
					CreatedAfter: photoTime.Format(time.RFC3339),
				}
				locations = make(map[string]struct{})
				lastLatLng = s2.LatLng{} // reset reference point for new event
			}
		}

		current.PhotoCount++
		lastPhotoTime = photoTime

		// Location tracking
		if source.Geo != nil && source.Geo.Available() {
			lastLocCheck := lastLocTime.Sub(photoTime)
			if lastLocCheck < 0 {
				lastLocCheck = -lastLocCheck
			}
			queryLocation := lastLocTime.IsZero() || lastLocCheck > time.Duration(locationGapTime)*time.Second
			if queryLocation && image.IsValidLatLng(info.LatLng) {
				lastLocTime = photoTime
				dist := image.AngleToKm(lastLatLng.Distance(info.LatLng))
				if dist > locationGapDist {
					location, err := source.Geo.ReverseGeocode(ctx, info.LatLng)
					if err == nil {
						locations[location] = struct{}{}
						lastLatLng = info.LatLng
					}
				}
			}
		}
	}

	// Finalize last event
	if current != nil && current.PhotoCount > 0 {
		current.CreatedBefore = lastPhotoTime.Format(time.RFC3339)
		current.Locations = make([]string, 0, len(locations))
		for loc := range locations {
			current.Locations = append(current.Locations, loc)
		}
		events = append(events, *current)
	}

	// Assign indices
	for i := range events {
		events[i].Index = i + 1
		events[i].LocationCount = len(events[i].Locations)
	}

	if events == nil {
		events = make([]EventSummary, 0)
	}

	return events, nil
}

// sameDay reports whether a and b are on the same calendar day.
func sameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
