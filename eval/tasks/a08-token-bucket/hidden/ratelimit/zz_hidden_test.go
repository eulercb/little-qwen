package ratelimit

import (
	"math"
	"sync"
	"testing"
	"time"
)

type zzhFakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (f *zzhFakeClock) Now() time.Time      { f.mu.Lock(); defer f.mu.Unlock(); return f.t }
func (f *zzhFakeClock) Add(d time.Duration) { f.mu.Lock(); f.t = f.t.Add(d); f.mu.Unlock() }

func zzhNear(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestHiddenBurstThenRefill(t *testing.T) {
	c := &zzhFakeClock{t: time.Unix(1000, 0)}
	l := New(2, 3, c)
	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Fatalf("burst %d denied", i)
		}
	}
	if l.Allow() {
		t.Fatal("should be empty")
	}
	c.Add(250 * time.Millisecond)
	if l.Allow() || !zzhNear(l.Tokens(), 0.5) {
		t.Fatalf("tokens = %v", l.Tokens())
	}
	c.Add(250 * time.Millisecond)
	if !l.Allow() {
		t.Fatal("refilled token denied")
	}
	c.Add(time.Hour)
	if !zzhNear(l.Tokens(), 3) {
		t.Fatalf("cap: %v", l.Tokens())
	}
}

func TestHiddenAllowN(t *testing.T) {
	c := &zzhFakeClock{t: time.Unix(0, 0)}
	l := New(1, 5, c)
	if l.AllowN(6) {
		t.Fatal("n > burst")
	}
	if !l.AllowN(0) || !zzhNear(l.Tokens(), 5) {
		t.Fatal("n=0")
	}
	if !l.AllowN(4) || l.AllowN(2) || !zzhNear(l.Tokens(), 1) {
		t.Fatalf("atomic AllowN: %v", l.Tokens())
	}
}

func TestHiddenReserve(t *testing.T) {
	c := &zzhFakeClock{t: time.Unix(0, 0)}
	l := New(4, 1, c)
	if d := l.Reserve(); d != 0 {
		t.Fatalf("first reserve %v", d)
	}
	if d := l.Reserve(); d != 250*time.Millisecond {
		t.Fatalf("second reserve %v", d)
	}
	if d := l.Reserve(); d != 500*time.Millisecond {
		t.Fatalf("third reserve %v", d)
	}
	if l.Allow() {
		t.Fatal("debt must block Allow")
	}
	c.Add(750 * time.Millisecond)
	if !zzhNear(l.Tokens(), 1) {
		t.Fatalf("after debt paid: %v", l.Tokens())
	}
}

func TestHiddenPanics(t *testing.T) {
	for _, f := range []func(){func() { New(0, 1, nil) }, func() { New(1, 0, nil) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("expected panic")
				}
			}()
			f()
		}()
	}
}

func TestHiddenConcurrent(t *testing.T) {
	c := &zzhFakeClock{t: time.Unix(0, 0)}
	l := New(1, 100, c)
	var wg sync.WaitGroup
	var mu sync.Mutex
	got := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if l.Allow() {
					mu.Lock()
					got++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	if got != 100 {
		t.Fatalf("allowed %d", got)
	}
}

func TestHiddenNilClock(t *testing.T) {
	l := New(1000, 1, nil)
	if !l.Allow() {
		t.Fatal("real clock")
	}
}
