package photofield

import (
	"log"
	"time"
)

func Elapsed(name string) func() {
	start := time.Now()
	return func() {
		log.Printf("%-20s %4d ms\n", name, time.Since(start).Milliseconds())
	}
}

func ElapsedWithCount(name string, count int) func() {
	start := time.Now()
	return func() {
		millis := time.Since(start).Milliseconds()
		log.Printf("%-20s %5d ms all, %.2f ms / photo\n", name, millis, float64(millis)/float64(count))
	}
}
