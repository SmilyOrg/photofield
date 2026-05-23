package pipeline

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// progress tracks and logs indexing progress with consistent formatting
type progress struct {
	mu          sync.Mutex
	name        string    // Stage name (e.g., "metadata", "thumbnails")
	total       int       // Total items to process (0 if unknown)
	processed   int       // Items completed
	lastLogged  int       // Processed count at last log
	lastLogTime time.Time // Time of last log
	logInterval time.Duration
	started     time.Time
	done        chan struct{}
	stopOnce    sync.Once
}

// NewProgress creates a new progress tracker
func newProgress(name string, total int) *progress {
	return &progress{
		name:        name,
		total:       total,
		logInterval: 2 * time.Second,
		started:     time.Now(),
		lastLogTime: time.Now(),
		done:        make(chan struct{}),
	}
}

// Inc increments the processed count by n
func (p *progress) Inc(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.processed += n

	now := time.Now()
	elapsed := now.Sub(p.lastLogTime)

	// Log periodically or when all items are processed
	shouldLog := elapsed >= p.logInterval || (p.total > 0 && p.processed >= p.total)

	if shouldLog {
		p.logProgress(now, elapsed)
	}
}

// logProgress logs current progress statistics (must be called with lock held)
func (p *progress) logProgress(now time.Time, elapsed time.Duration) {
	newlyProcessed := p.processed - p.lastLogged
	rate := float64(newlyProcessed) / elapsed.Seconds()

	if p.total > 0 {
		// Known total - show percentage and time left
		pending := p.total - p.processed
		percent := 100
		if p.total > 0 {
			percent = p.processed * 100 / p.total
		}

		rateDiv := 1
		if rate > 1 {
			rateDiv = int(rate)
		}

		var timeLeft time.Duration
		if rateDiv > 0 {
			timeLeft = time.Duration(pending/rateDiv) * time.Second
		}

		log.Printf("index %s %4d%% completed, %5d loaded, %5d pending, %.2f / sec, %s left\n",
			p.name, percent, p.processed, pending, rate, timeLeft)
	} else {
		// Unknown total - just show processed count and rate
		log.Printf("index %s %5d loaded, %.2f / sec\n",
			p.name, p.processed, rate)
	}

	p.lastLogged = p.processed
	p.lastLogTime = now
}

// Done completes the progress tracking and logs final statistics
func (p *progress) Done() {
	p.stopOnce.Do(func() {
		close(p.done)

		p.mu.Lock()
		defer p.mu.Unlock()

		elapsed := time.Since(p.started)
		rate := float64(p.processed) / elapsed.Seconds()

		log.Printf("index %s completed %d files, %.2f / sec, %s total\n",
			p.name, p.processed, rate, elapsed.Round(time.Millisecond))
	})
}

type counter struct {
	name  string
	value int
}

// multiProgress tracks progress for multiple sub-operations in a single stage
type multiProgress struct {
	mu          sync.Mutex
	name        string    // Stage name (e.g., "contents")
	total       int       // Total items to process
	processed   int       // Items completed
	counters    []counter // Sub-counters (e.g., {"color", 123}, {"embedding", 100})
	lastLogged  int       // Processed count at last log
	lastLogTime time.Time // Time of last log
	logInterval time.Duration
	started     time.Time
	done        chan struct{}
	stopOnce    sync.Once
}

// NewMultiProgress creates a progress tracker with multiple counters
func newMultiProgress(name string, total int, counterNames ...string) *multiProgress {
	counters := make([]counter, len(counterNames))
	for i, name := range counterNames {
		counters[i] = counter{name: name}
	}

	return &multiProgress{
		name:        name,
		total:       total,
		counters:    counters,
		logInterval: 2 * time.Second,
		started:     time.Now(),
		lastLogTime: time.Now(),
		done:        make(chan struct{}),
	}
}

// Inc increments the main processed count
func (mp *multiProgress) Inc(n int) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	mp.processed += n

	now := time.Now()
	elapsed := now.Sub(mp.lastLogTime)

	shouldLog := elapsed >= mp.logInterval || (mp.total > 0 && mp.processed >= mp.total)

	if shouldLog {
		mp.logProgress(now, elapsed)
	}
}

// IncCounter increments a named sub-counter
func (mp *multiProgress) IncCounter(name string, n int) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	for i := range mp.counters {
		if mp.counters[i].name == name {
			mp.counters[i].value += n
			return
		}
	}
}

// logProgress logs current progress with all sub-counters
func (mp *multiProgress) logProgress(now time.Time, elapsed time.Duration) {
	newlyProcessed := mp.processed - mp.lastLogged
	rate := float64(newlyProcessed) / elapsed.Seconds()

	if mp.total > 0 {
		pending := mp.total - mp.processed
		percent := 100
		if mp.total > 0 {
			percent = mp.processed * 100 / mp.total
		}

		rateDiv := 1
		if rate > 1 {
			rateDiv = int(rate)
		}

		var timeLeft time.Duration
		if rateDiv > 0 {
			timeLeft = time.Duration(pending/rateDiv) * time.Second
		}

		log.Printf("index %s %4d%% completed, %5d loaded, %5d pending, %.2f / sec, %s left%s\n",
			mp.name, percent, mp.processed, pending, rate, timeLeft, mp.counterStr())
	} else {
		log.Printf("index %s %5d loaded, %.2f / sec%s\n",
			mp.name, mp.processed, rate, mp.counterStr())
	}

	mp.lastLogged = mp.processed
	mp.lastLogTime = now
}

// counterStr builds an ordered counter summary string for log output
func (mp *multiProgress) counterStr() string {
	if len(mp.counters) == 0 {
		return ""
	}
	s := " ("
	for i, c := range mp.counters {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%s: %d", c.name, c.value)
	}
	return s + ")"
}

// Done completes the progress tracking and logs final statistics
func (mp *multiProgress) Done() {
	mp.stopOnce.Do(func() {
		close(mp.done)

		mp.mu.Lock()
		defer mp.mu.Unlock()

		elapsed := time.Since(mp.started)
		rate := float64(mp.processed) / elapsed.Seconds()

		log.Printf("index %s completed %d files, %.2f / sec, %s total%s\n",
			mp.name, mp.processed, rate, elapsed.Round(time.Millisecond), mp.counterStr())
	})
}
