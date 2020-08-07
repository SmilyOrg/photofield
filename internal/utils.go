package photofield

import (
	"log"
	"time"
)

func Elapsed(name string) func() {
	start := time.Now()
	return func() {
		log.Printf("%-20s %5d ms\n", name, time.Since(start).Milliseconds())
	}
}

func ElapsedWithCount(name string, count int) func() {
	start := time.Now()
	return func() {
		millis := time.Since(start).Milliseconds()
		log.Printf("%-20s %5d ms all, %.2f ms / photo\n", name, millis, float64(millis)/float64(count))
	}
}

func SameDay(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
