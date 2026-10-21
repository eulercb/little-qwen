package pool

import (
	"context"
	"errors"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestHiddenOrder(t *testing.T) {
	in := make([]int, 200)
	for i := range in {
		in[i] = i
	}
	out, err := Map(context.Background(), in, 8, func(_ context.Context, x int) (int, error) {
		time.Sleep(time.Duration(rand.Intn(300)) * time.Microsecond)
		return x * 2, nil
	})
	if err != nil || len(out) != 200 {
		t.Fatal(len(out), err)
	}
	for i, v := range out {
		if v != 2*i {
			t.Fatalf("out[%d]=%d", i, v)
		}
	}
}

func TestHiddenLowestIndexError(t *testing.T) {
	boom := errors.New("boom")
	var started atomic.Int64
	_, err := Map(context.Background(), make([]int, 200), 4, func(_ context.Context, _ int) (int, error) {
		if started.Add(1) == 5 {
			return 0, boom
		}
		time.Sleep(2 * time.Millisecond)
		return 0, nil
	})
	var ie *IndexError
	if !errors.As(err, &ie) || !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	// After the failure, at most a few already-dispatched calls may start.
	if n := started.Load(); n > 5+4+4 {
		t.Fatalf("kept starting work after failure: %d calls", n)
	}
}

func TestHiddenLowestIndexDeterministic(t *testing.T) {
	// Input 7 fails fast, input 2 fails slowly: the reported index must be 2.
	in := []int{0, 1, 2, 3, 4, 5, 6, 7}
	_, err := Map(context.Background(), in, 8, func(_ context.Context, x int) (int, error) {
		switch x {
		case 2:
			time.Sleep(50 * time.Millisecond)
			return 0, errors.New("slow")
		case 7:
			return 0, errors.New("fast")
		}
		return x, nil
	})
	var ie *IndexError
	if !errors.As(err, &ie) || ie.Index != 2 {
		t.Fatalf("err=%v", err)
	}
}

func TestHiddenCancelNoHangNoLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Map(ctx, make([]int, 1000), 4, func(ctx context.Context, _ int) (int, error) {
			select {
			case <-ctx.Done():
				return 0, nil
			case <-time.After(5 * time.Millisecond):
				return 1, nil
			}
		})
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Map hung after cancel")
	}
	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+1 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestHiddenEarlyErrorNoLeak(t *testing.T) {
	before := runtime.NumGoroutine()
	for k := 0; k < 20; k++ {
		Map(context.Background(), make([]int, 50), 4, func(_ context.Context, _ int) (int, error) { return 0, errors.New("x") })
	}
	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+1 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
}

func TestHiddenEmptyAndZeroWorkers(t *testing.T) {
	out, err := Map(context.Background(), []int{}, 0, func(_ context.Context, x int) (int, error) { return x, nil })
	if err != nil || len(out) != 0 {
		t.Fatal(out, err)
	}
	out, err = Map(context.Background(), []int{5}, 0, func(_ context.Context, x int) (int, error) { return x, nil })
	if err != nil || out[0] != 5 {
		t.Fatal(out, err)
	}
}
