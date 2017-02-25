package main

import "github.com/prometheus/client_golang/prometheus"

var (
	getCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_get_op",
			Help: "Number of Get operations.",
		},
		[]string{"operation"},
	)
	createCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_create_op",
			Help: "Number of Create operations.",
		},
		[]string{"operation"},
	)
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(getCounter)
	prometheus.MustRegister(createCounter)
}
