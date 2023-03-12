package queue

import (
	"log"
	"photofield/internal/metrics"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sheerun/queue"
)

type Queue struct {
	queue       *queue.Queue
	ID          string
	Name        string
	Worker      func(<-chan interface{})
	WorkerCount int
}

func (q *Queue) Run() {
	if q.queue == nil {
		q.queue = queue.New()
	}

	loadCount := 0
	lastLoadCount := 0
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	metrics.AddQueue(q.ID, q.queue)
	var doneCounter = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Name:      q.ID + "_done",
	})

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
			item := q.queue.Pop()
			if item == nil {
				log.Printf("%s queue stopping\n", q.Name)
				return
			}
			items <- item
		}
		doneCounter.Inc()

		now := time.Now()
		elapsed := now.Sub(lastLogTime)
		if elapsed > logInterval || q.queue.Length() == 0 {
			perSec := float64(loadCount-lastLoadCount) / elapsed.Seconds()
			pendingCount := q.queue.Length()
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
			log.Printf("%s queue %5d pending\n", q.Name, q.queue.Length())
		}
	}
}

func (q *Queue) Length() int {
	if q.queue == nil {
		return 0
	}
	return q.queue.Length()
}

func (q *Queue) AppendItems(items <-chan interface{}) {
	if q.queue == nil {
		return
	}
	for item := range items {
		q.queue.Append(item)
	}
}
