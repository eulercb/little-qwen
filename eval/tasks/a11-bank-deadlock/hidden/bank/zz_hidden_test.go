package bank

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestHiddenRingNoDeadlock(t *testing.T) {
	b := New(500, 500, 500, 500, 500)
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			for d := 1; d <= 4; d++ {
				wg.Add(1)
				go func(i, d int) {
					defer wg.Done()
					for j := 0; j < 300; j++ {
						b.Transfer(i, (i+d)%5, 2)
					}
				}(i, d)
			}
		}
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("deadlock")
	}
	if b.Total() != 2500 {
		t.Fatalf("total %d", b.Total())
	}
}

func TestHiddenTotalConsistentSnapshot(t *testing.T) {
	b := New(1000, 1000, 1000)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					b.Transfer(i, (i+1)%3, 7)
					b.Transfer((i+2)%3, i, 5)
				}
			}
		}(i)
	}
	bad := 0
	for k := 0; k < 3000; k++ {
		if b.Total() != 3000 {
			bad++
		}
	}
	close(stop)
	wg.Wait()
	if bad > 0 {
		t.Fatalf("Total observed inconsistent sum %d times", bad)
	}
}

func TestHiddenErrorsKept(t *testing.T) {
	b := New(10, 10)
	cases := []struct {
		from, to int
		amt      int64
		want     error
	}{{0, 0, 1, ErrSameAccount}, {0, 1, 0, ErrBadAmount}, {0, 1, -5, ErrBadAmount}, {0, 9, 1, ErrNoAccount}, {9, 0, 1, ErrNoAccount}, {0, 1, 11, ErrInsufficient}}
	for _, c := range cases {
		if err := b.Transfer(c.from, c.to, c.amt); !errors.Is(err, c.want) {
			t.Errorf("%+v: got %v", c, err)
		}
	}
	if b.Total() != 20 {
		t.Fatal("total changed")
	}
}
