package main

import (
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/uber-go/tally"
)

func TestZKPacketMetricsNewRootScope_OK(t *testing.T) {
	scopeFacotry := func() (tally.Scope, tally.CachedStatsReporter, io.Closer, error) {
		return tally.NoopScope, NopCachedStatsReporter, ioutil.NopCloser(nil), nil
	}
}

var (
	capabilitiesReportingNoTagging = &capabilities{
		reporting: true,
		tagging:   false,
	}
)

type capabilities struct {
	reporting bool
	tagging   bool
}

func (c *capabilities) Reporting() bool {
	return c.reporting
}

func (c *capabilities) Tagging() bool {
	return c.tagging
}

// NopCachedStatsReporter is an implementatin of tally.CachedStatsReporter than simply does nothing.
// TODO:(anup) This should exist in tally. https://github.com/uber-go/tally/issues/23
// Remove and replace metrics.NopCachedStatsReporter with tally.NopCachedStatsRepor once issue is resolved
var NopCachedStatsReporter tally.CachedStatsReporter = nopCachedStatsReporter{}

type nopCachedStatsReporter struct {
}

func (nopCachedStatsReporter) AllocateCounter(name string, tags map[string]string) tally.CachedCount {
	return NopCachedCount
}

func (nopCachedStatsReporter) AllocateGauge(name string, tags map[string]string) tally.CachedGauge {
	return NopCachedGauge
}

func (nopCachedStatsReporter) AllocateTimer(name string, tags map[string]string) tally.CachedTimer {
	return NopCachedTimer
}

func (r nopCachedStatsReporter) Capabilities() tally.Capabilities {
	return capabilitiesReportingNoTagging
}

func (r nopCachedStatsReporter) Flush() {
}

// NopCachedCount is an implementation of tally.CachedCount
var NopCachedCount tally.CachedCount = nopCachedCount{}

type nopCachedCount struct {
}

func (nopCachedCount) ReportCount(value int64) {
}

// NopCachedGauge is an implementation of tally.CachedGauge
var NopCachedGauge tally.CachedGauge = nopCachedGauge{}

type nopCachedGauge struct {
}

func (nopCachedGauge) ReportGauge(value float64) {
}

// NopCachedTimer is an implementation of tally.CachedTimer
var NopCachedTimer tally.CachedTimer = nopCachedTimer{}

type nopCachedTimer struct {
}

func (nopCachedTimer) ReportTimer(interval time.Duration) {
}
