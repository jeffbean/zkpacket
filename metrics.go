package main

import "github.com/prometheus/client_golang/prometheus"

var (
	operationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_op_count",
			Help: "Number of operations.",
		},
		[]string{"operation"},
	)
	operationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zk_op_duration_ms",
			Help:    "The time for a given operation operation.",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 10 buckets, each increasing by a factor of 2.
		},
		[]string{"operation"},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(operationCounter)
	prometheus.MustRegister(operationHistogram)
}
