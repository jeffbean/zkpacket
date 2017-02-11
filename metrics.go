package main

import (
	"time"

	promreporter "github.com/uber-go/tally/prometheus"

	"github.com/uber-go/tally"
)

func setupMetrics() error {
	r := promreporter.NewReporter(promreporter.Options{})
	// Note: `promreporter.DefaultSeparator` is "_".
	// Prometheus doesnt like metrics with "." or "-" in them.
	scope, closer := tally.NewCachedRootScope("zkpacket", map[string]string{}, r, 1*time.Second, promreporter.DefaultSeparator)
	defer closer.Close()

}
