package pool

import (
	"context"
	"fmt"
	"sync"
)

type IndexError struct {
	Index int
	Err   error
}

func (e *IndexError) Error() string { return fmt.Sprintf("pool: input %d: %v", e.Index, e.Err) }
func (e *IndexError) Unwrap() error { return e.Err }

func Map[T, R any](ctx context.Context, in []T, workers int, fn func(context.Context, T) (R, error)) ([]R, error) {
	if workers < 1 {
		workers = 1
	}
	out := make([]R, len(in))
	var mu sync.Mutex
	failIdx := -1
	var failErr error
	stop := make(chan struct{})
	var once sync.Once
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				r, err := fn(ctx, in[i])
				mu.Lock()
				if err != nil {
					if failIdx < 0 || i < failIdx {
						failIdx, failErr = i, err
					}
					once.Do(func() { close(stop) })
				} else {
					out[i] = r
				}
				mu.Unlock()
			}
		}()
	}
	cancelled := false
feed:
	for i := range in {
		select {
		case <-stop:
			break feed
		case <-ctx.Done():
			cancelled = true
			break feed
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	if failIdx >= 0 {
		return nil, &IndexError{Index: failIdx, Err: failErr}
	}
	if cancelled || ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return out, nil
}
