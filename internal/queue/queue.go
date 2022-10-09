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
	Worker      func(<-chan uint32)
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

	ids := make(chan uint32)
	defer close(ids)

	if q.WorkerCount == 0 {
		q.WorkerCount = 1
	}
	for i := 0; i < q.WorkerCount; i++ {
		go q.Worker(ids)
	}

	for {
		id := q.queue.Pop().(uint32)
		if id == 0 {
			log.Printf("%s queue stopping\n", q.Name)
			return
		}

		ids <- id
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
			log.Printf("%s queue id %5d, %5d pending\n", q.Name, id, q.queue.Length())
		}
	}
}

func (q *Queue) Append(id uint32) {
	if q.queue == nil {
		return
	}
	q.queue.Append(id)
}

func (q *Queue) AppendChan(ids <-chan uint32) {
	if q.queue == nil {
		return
	}
	for id := range ids {
		q.queue.Append(id)
	}
}
