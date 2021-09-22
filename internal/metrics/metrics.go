package metrics

import (
	"github.com/dgraph-io/ristretto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sheerun/queue"
)

var Namespace = "pf"

func addGauge(name string, function func() float64) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      name,
	},
		function,
	)
}

func addCounterUint64(name string, function func() uint64) {
	promauto.NewCounterFunc(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      name,
	},
		func() float64 { return float64(function()) },
	)
}

func addGaugeUint64(name string, function func() uint64) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      name,
	},
		func() float64 { return float64(function()) },
	)
}

func AddQueue(name string, queue *queue.Queue) {
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      name + "_pending",
	}, func() float64 {
		return float64(queue.Length())
	})
}

func AddRistretto(name string, cache *ristretto.Cache) {
	addGauge(name+"_ratio", cache.Metrics.Ratio)
	addCounterUint64(name+"_hits", cache.Metrics.Hits)
	addCounterUint64(name+"_misses", cache.Metrics.Misses)
	addCounterUint64(name+"_cost_added", cache.Metrics.CostAdded)
	addCounterUint64(name+"_cost_evicted", cache.Metrics.CostEvicted)
	addGaugeUint64(name+"_cost_active", func() uint64 {
		return cache.Metrics.CostAdded() - cache.Metrics.CostEvicted()
	})
	addCounterUint64(name+"_keys_added", cache.Metrics.KeysAdded)
	addCounterUint64(name+"_keys_evicted", cache.Metrics.KeysEvicted)
	addCounterUint64(name+"_keys_updated", cache.Metrics.KeysUpdated)
	addGaugeUint64(name+"_keys_active", func() uint64 {
		return cache.Metrics.KeysAdded() - cache.Metrics.KeysEvicted()
	})
	addCounterUint64(name+"_sets_dropped", cache.Metrics.SetsDropped)
	addCounterUint64(name+"_sets_rejected", cache.Metrics.SetsRejected)
	addCounterUint64(name+"_gets_kept", cache.Metrics.GetsKept)
	addCounterUint64(name+"_gets_dropped", cache.Metrics.GetsDropped)
}
