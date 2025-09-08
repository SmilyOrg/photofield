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
	Stop        chan bool
}

func (q *Queue) Run() {
	q.mu.Lock()
	if q.queue == nil {
		q.queue = queue.New()
		q.Stop = make(chan bool)
	}
	queue := q.queue
	stop := q.Stop
	q.mu.Unlock()

	loadCount := 0
	lastLoadCount := 0
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	m := metrics.AddQueue(q.ID, queue)

	logging := false

	items := make(chan interface{})
	defer close(items)

	if q.WorkerCount == 0 {
		q.WorkerCount = 1
	}
	for i := 0; i < q.WorkerCount; i++ {
		if q.Worker != nil {
			go q.Worker(items)
		}
	}

	for {
		if q.Worker != nil {
			item := queue.Pop()
			if item == nil {
				log.Printf("%s queue stopping\n", q.Name)
				close(stop)
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
			// log.Printf("%s %4d%% completed, %5d loaded, %5d pending, %.2f / sec\n", q.Name, percent, loadCount, pendingCount, perSec)
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
			// log.Printf("image info load for id %5d, %5d pending, %5d ms get file, %5d ms set db, %5d ms set cache\n", id, len(backlog), fileGetMs, dbSetMs, cacheSetMs)
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
	stop := q.Stop
	q.mu.Unlock()

	queue.Clean()
	queue.Append(nil)
	<-stop

	q.mu.Lock()
	q.queue = nil
	q.mu.Unlock()
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
