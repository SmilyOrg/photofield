package image

import (
	"log"
	"photofield/internal/metrics"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sheerun/queue"
)

func (source *Source) LoadInfo(path string) (Info, error) {
	var info Info
	var err error
	err = source.decoder.DecodeInfo(path, &info)
	if err != nil {
		return info, err
	}

	color, err := source.LoadImageColor(path)
	if err != nil {
		return info, err
	}
	info.SetColorRGBA(color)

	return info, nil
}

func (source *Source) LoadInfoMeta(path string) (Info, error) {
	var info Info
	err := source.decoder.DecodeInfo(path, &info)
	if err != nil {
		return info, err
	}
	return info, nil
}

func (source *Source) LoadInfoColor(path string) (Info, error) {
	var info Info
	color, err := source.LoadImageColor(path)
	if err != nil {
		return info, err
	}
	info.SetColorRGBA(color)
	return info, nil
}

func (source *Source) processQueue(name string, id string, queue *queue.Queue, workerFn func(<-chan string), workerCount int) {

	loadCount := 0
	lastLoadCount := 0
	lastLogTime := time.Now()
	logInterval := 2 * time.Second
	metrics.AddQueue(id, queue)
	var doneCounter = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: metrics.Namespace,
		Name:      id + "_done",
	})

	logging := false

	paths := make(chan string)
	defer close(paths)

	for i := 0; i < workerCount; i++ {
		go workerFn(paths)
	}

	for {
		id := queue.Pop().(ImageId)
		if id == 0 {
			log.Printf("%s queue stopping\n", name)
			return
		}

		path, err := source.GetImagePath(id)
		if err != nil {
			panic("Unable to load image info for non-existing image id")
		}
		paths <- path
		doneCounter.Inc()

		now := time.Now()
		elapsed := now.Sub(lastLogTime)
		if elapsed > logInterval || queue.Length() == 0 {
			perSec := float64(loadCount-lastLoadCount) / elapsed.Seconds()
			pendingCount := queue.Length()
			percent := 100
			if loadCount+pendingCount > 0 {
				percent = loadCount * 100 / (loadCount + pendingCount)
			}
			log.Printf("%s %4d%% completed, %5d loaded, %5d pending, %.2f / sec\n", name, percent, loadCount, pendingCount, perSec)
			lastLoadCount = loadCount
			lastLogTime = now
		}

		loadCount++

		if logging {
			// log.Printf("image info load for id %5d, %5d pending, %5d ms get file, %5d ms set db, %5d ms set cache\n", id, len(backlog), fileGetMs, dbSetMs, cacheSetMs)
			log.Printf("%s queue id %5d, %5d pending\n", name, id, queue.Length())
		}
	}
}
