package main

import (
	"fmt"
	"io"
	"time"

	"github.com/uber-go/tally"
	promreporter "github.com/uber-go/tally/prometheus"
)

type rootScopeFactory func() (tally.Scope, tally.CachedStatsReporter, io.Closer, error)

// RootScope returns the provided metrics scope and stats reporter from the given factory
func RootScope() (tally.Scope, tally.CachedStatsReporter, io.Closer) {
	return newRootScope(getRootScope)
}

func newRootScope(scopeFactory rootScopeFactory) (tally.Scope, tally.CachedStatsReporter, io.Closer) {
	scope, reporter, closer, err := scopeFactory()
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize metrics reporter %v", err))
	}
	return scope, reporter, closer
}

func getRootScope() (tally.Scope, tally.CachedStatsReporter, io.Closer, error) {
	reporter := promreporter.NewReporter(promreporter.Options{})
	scope, closer := tally.NewCachedRootScope("zkpacket",
		map[string]string{},
		reporter,
		1*time.Second,
		promreporter.DefaultSeparator,
	)
	return scope, reporter, closer, nil
}
