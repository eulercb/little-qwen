// Package stats collects simple named counters and latency samples.
package stats

import (
	"sort"
	"sync"
	"time"
)

// Collector aggregates counters and durations.
type Collector struct {
	mu       sync.Mutex
	counters map[string]int64
	samples  map[string][]time.Duration
	hooks    []func(name string, v int64)
}

// New returns an empty Collector.
func New() *Collector {
	return &Collector{counters: map[string]int64{}, samples: map[string][]time.Duration{}}
}

// OnIncr registers a hook called after every Incr with the new value.
func (c *Collector) OnIncr(f func(name string, v int64)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, f)
}

// Incr adds delta to the named counter and returns the new value.
func (c *Collector) Incr(name string, delta int64) int64 {
	c.mu.Lock()
	c.counters[name] += delta
	v := c.counters[name]
	hooks := append([]func(string, int64){}, c.hooks...)
	c.mu.Unlock()
	for _, h := range hooks {
		h(name, v)
	}
	return v
}

// Get returns the counter value.
func (c *Collector) Get(name string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counters[name]
}

// Observe records a duration sample.
func (c *Collector) Observe(name string, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.samples[name] = append(c.samples[name], d)
}

// Percentile returns the p-th percentile (0..100) of the samples for name
// using nearest-rank, or 0 when there are none.
func (c *Collector) Percentile(name string, p float64) time.Duration {
	c.mu.Lock()
	s := append([]time.Duration(nil), c.samples[name]...)
	c.mu.Unlock()
	if len(s) == 0 {
		return 0
	}
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	idx := int(p/100*float64(len(s))+0.5) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s) {
		idx = len(s) - 1
	}
	return s[idx]
}

// Snapshot returns a copy of all counters.
func (c *Collector) Snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.counters))
	for k, v := range c.counters {
		out[k] = v
	}
	return out
}

// Reset clears everything except hooks.
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters = map[string]int64{}
	c.samples = map[string][]time.Duration{}
}
