package stats

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHiddenConcurrentMixed(t *testing.T) {
	c := New()
	var hookCalls atomic.Int64
	c.OnIncr(func(string, int64) { hookCalls.Add(1) })
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 300; j++ {
				c.Incr("k", 1)
				c.Observe("lat", time.Duration(j)*time.Microsecond)
				_ = c.Percentile("lat", 90)
				_ = c.Snapshot()
				_ = c.Get("k")
			}
		}(i)
	}
	wg.Wait()
	if c.Get("k") != 1800 || hookCalls.Load() != 1800 {
		t.Fatalf("k=%d hooks=%d", c.Get("k"), hookCalls.Load())
	}
}

func TestHiddenHookCanReadCollector(t *testing.T) {
	// A hook that reads the collector must not deadlock.
	c := New()
	var seen int64
	c.OnIncr(func(name string, v int64) { seen = c.Get(name) })
	done := make(chan struct{})
	go func() { c.Incr("x", 4); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: hook calling Get blocked")
	}
	if seen != 4 {
		t.Fatalf("seen=%d", seen)
	}
}

func TestHiddenResetConcurrent(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); for j := 0; j < 200; j++ { c.Incr("a", 1) } }()
		go func() { defer wg.Done(); for j := 0; j < 20; j++ { c.Reset() } }()
	}
	wg.Wait()
	c.Reset()
	if len(c.Snapshot()) != 0 {
		t.Fatal("not reset")
	}
}

func TestHiddenPercentileDoesNotMutate(t *testing.T) {
	c := New()
	for _, ms := range []int{5, 1, 3} {
		c.Observe("x", time.Duration(ms)*time.Millisecond)
	}
	if c.Percentile("x", 100) != 5*time.Millisecond || c.Percentile("x", 1) != time.Millisecond {
		t.Fatal("percentile")
	}
}
