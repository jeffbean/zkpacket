package main

import "github.com/prometheus/client_golang/prometheus"

var (
	operationCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_op_count",
			Help: "Number of operations.",
		},
		[]string{"operation", "direction"},
	)
	operationHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "zk_op_seconds",
			Help: "The time for a given operation operation.",
		},
		[]string{"operation"},
	)
	packetSizeHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "packet_size",
			Help:    "The raw packetsize given in the first 4 bytes of each packet",
			Buckets: prometheus.ExponentialBuckets(2 /* start */, 2 /* factor */, 10 /* count */),
		},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(operationCounter)
	prometheus.MustRegister(operationHistogram)
	prometheus.MustRegister(packetSizeHistogram)
}
