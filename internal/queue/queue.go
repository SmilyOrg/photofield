package queue

import (
	"log"
	"photofield/internal/metrics"
	"sync"
	"time"

	"github.com/sheerun/queue"
)

type Queue struct {
	mu          sync.RWMutex
	queue       *queue.Queue
	ID          string
	Name        string
	Worker      func(<-chan interface{})
	WorkerCount int
	workerWg    sync.WaitGroup
}

func (q *Queue) Run() {
	q.mu.Lock()
	if q.queue == nil {
		q.queue = queue.New()
	}
	queue := q.queue
	q.mu.Unlock()

	loadCount := 0
	lastLoadCount := 0
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	m := metrics.AddQueue(q.ID, queue)

	logging := false

	items := make(chan interface{})

	if q.WorkerCount == 0 {
		q.WorkerCount = 1
	}

	// Start workers with WaitGroup tracking
	for i := 0; i < q.WorkerCount; i++ {
		if q.Worker != nil {
			q.workerWg.Add(1)
			go func() {
				defer q.workerWg.Done()
				q.Worker(items)
			}()
		}
	}

	for {
		if q.Worker != nil {
			item := queue.Pop()
			if item == nil {
				log.Printf("%s queue stopping\n", q.Name)
				close(items)
				q.workerWg.Wait() // Wait for all workers to finish
				return
			}
			items <- item
		}
		m.Done.Inc()

		now := time.Now()
		elapsed := now.Sub(lastLogTime)
		if elapsed > logInterval || queue.Length() == 0 {
			perSec := float64(loadCount-lastLoadCount) / elapsed.Seconds()
			pendingCount := queue.Length()
			percent := 100
			if loadCount+pendingCount > 0 {
				percent = loadCount * 100 / (loadCount + pendingCount)
			}
			perSecDiv := 1
			if perSec > 1 {
				perSecDiv = int(perSec)
			}
			timeLeft := time.Duration(pendingCount/perSecDiv) * time.Second
			log.Printf("%s %4d%% completed, %5d loaded, %5d pending, %.2f / sec, %s left\n", q.Name, percent, loadCount, pendingCount, perSec, timeLeft)
			lastLoadCount = loadCount
			lastLogTime = now
		}

		loadCount++

		if logging {
			log.Printf("%s queue %5d pending\n", q.Name, queue.Length())
		}
	}
}

func (q *Queue) Wait() {
	for {
		q.mu.RLock()
		queue := q.queue
		q.mu.RUnlock()
		if queue == nil || queue.Length() == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (q *Queue) Close() {
	if q == nil {
		return
	}

	q.mu.Lock()
	if q.queue == nil {
		q.mu.Unlock()
		return
	}
	queue := q.queue
	q.mu.Unlock()

	queue.Clean()
	queue.Append(nil)

	q.workerWg.Wait()

	q.mu.Lock()
	q.queue = nil
	q.mu.Unlock()
}

func (q *Queue) WaitForWorkers() {
	q.workerWg.Wait()
}

func (q *Queue) Length() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.queue == nil {
		return 0
	}
	return q.queue.Length()
}

func (q *Queue) AppendItems(items <-chan interface{}) {
	q.mu.RLock()
	queue := q.queue
	q.mu.RUnlock()

	if queue == nil {
		return
	}
	for item := range items {
		queue.Append(item)
	}
}
