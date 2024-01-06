package metrics

import (
	"sync"

	"github.com/dgraph-io/ristretto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sheerun/queue"
)

var Namespace = "pf"

var mutex sync.Mutex
var gauges = make(map[string]prometheus.GaugeFunc)
var counters = make(map[string]prometheus.CounterFunc)
var manualCounters = make(map[string]prometheus.Counter)
var histograms = make(map[string]*prometheus.HistogramVec)

func addGauge(name string, function func() float64) {
	if _, ok := gauges[name]; ok {
		// Gauge already exists, remove it before adding a new one
		prometheus.Unregister(gauges[name])
	}
	gauge := promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      name,
	}, function)
	gauges[name] = gauge
}

func addCounter(name string, function func() float64) {
	if _, ok := counters[name]; ok {
		// Counter already exists, remove it before adding a new one
		prometheus.Unregister(counters[name])
	}
	counter := promauto.NewCounterFunc(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      name,
	}, function)
	counters[name] = counter
}

func addCounterUint64(name string, function func() uint64) {
	addCounter(name, func() float64 {
		return float64(function())
	})
}

func addManualCounter(name string) prometheus.Counter {
	if _, ok := manualCounters[name]; ok {
		// Counter already exists, remove it before adding a new one
		prometheus.Unregister(manualCounters[name])
	}
	counter := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      name,
	})
	manualCounters[name] = counter
	return counter
}

type Queue struct {
	Done prometheus.Counter
}

func AddQueue(name string, queue *queue.Queue) Queue {
	mutex.Lock()
	defer mutex.Unlock()
	addGauge(name+"_pending", func() float64 {
		return float64(queue.Length())
	})
	return Queue{
		Done: addManualCounter(name + "_done"),
	}
}

func AddRistretto[K any, V any](name string, cache *ristretto.Cache[K, V]) {
	mutex.Lock()
	defer mutex.Unlock()
	addGauge(name+"_ratio", cache.Metrics.Ratio)
	addCounterUint64(name+"_hits", cache.Metrics.Hits)
	addCounterUint64(name+"_misses", cache.Metrics.Misses)
	addCounterUint64(name+"_cost_added", cache.Metrics.CostAdded)
	addCounterUint64(name+"_cost_evicted", cache.Metrics.CostEvicted)
	addGauge(name+"_cost_active", func() float64 {
		return float64(cache.Metrics.CostAdded() - cache.Metrics.CostEvicted())
	})
	addCounterUint64(name+"_keys_added", cache.Metrics.KeysAdded)
	addCounterUint64(name+"_keys_evicted", cache.Metrics.KeysEvicted)
	addCounterUint64(name+"_keys_updated", cache.Metrics.KeysUpdated)
	addGauge(name+"_keys_active", func() float64 {
		return float64(cache.Metrics.KeysAdded() - cache.Metrics.KeysEvicted())
	})
	addCounterUint64(name+"_sets_dropped", cache.Metrics.SetsDropped)
	addCounterUint64(name+"_sets_rejected", cache.Metrics.SetsRejected)
	addCounterUint64(name+"_gets_kept", cache.Metrics.GetsKept)
	addCounterUint64(name+"_gets_dropped", cache.Metrics.GetsDropped)
}

func AddHistogram(name string, buckets []float64, labelNames []string) *prometheus.HistogramVec {
	mutex.Lock()
	defer mutex.Unlock()
	if existingHistogram, ok := histograms[name]; ok {
		// Histogram already exists, unregister it before adding a new one
		prometheus.Unregister(existingHistogram)
	}
	histogram := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      name,
		Buckets:   buckets,
	}, labelNames)
	histograms[name] = histogram
	return histogram
}
