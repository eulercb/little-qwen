package stats

import (
	"sync"
	"testing"
	"time"
)

func TestIncr(t *testing.T) {
	c := New()
	c.Incr("a", 2)
	if c.Incr("a", 3) != 5 || c.Get("a") != 5 {
		t.Fatal("bad counter")
	}
}

func TestPercentile(t *testing.T) {
	c := New()
	for i := 1; i <= 100; i++ {
		c.Observe("x", time.Duration(i)*time.Millisecond)
	}
	if got := c.Percentile("x", 50); got != 50*time.Millisecond {
		t.Fatalf("p50 = %v", got)
	}
}

func TestConcurrentIncr(t *testing.T) {
	c := New()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				c.Incr("n", 1)
			}
		}()
	}
	wg.Wait()
	if c.Get("n") != 8000 {
		t.Fatalf("n = %d", c.Get("n"))
	}
}
