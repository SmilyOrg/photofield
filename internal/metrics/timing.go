package metrics

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

type Counter struct {
	Name      string
	Interval  time.Duration
	lastTime  time.Time
	lastValue int
}

func (counter *Counter) Set(value int) {
	now := time.Now()
	elapsed := now.Sub(counter.lastTime)
	if elapsed >= counter.Interval {
		speed := float64(value-counter.lastValue) / elapsed.Seconds()
		if !counter.lastTime.IsZero() {
			log.Printf("%v %7v, %0.2f / sec\n", counter.Name, value, speed)
		}
		counter.lastTime = now
		counter.lastValue = value
	}
}
