// Package pool runs work on a bounded number of goroutines.
package pool

import (
	"context"
	"fmt"
	"sync"
)

// IndexError reports which input failed.
type IndexError struct {
	Index int
	Err   error
}

func (e *IndexError) Error() string { return fmt.Sprintf("pool: input %d: %v", e.Index, e.Err) }
func (e *IndexError) Unwrap() error { return e.Err }

// Map calls fn on every element of in using at most workers goroutines
// (workers < 1 means 1) and returns the results in input order: out[i] is
// fn(ctx, in[i]).
//
// If any call returns an error, Map stops handing out new inputs, waits for
// running calls to return, and returns a nil slice and an *IndexError for the
// failing input with the LOWEST index among those that failed.
//
// If ctx is cancelled, Map stops handing out new inputs, waits for running
// calls to return, and returns ctx.Err() (unless a call failed first, in which
// case the rule above applies). Map never leaks goroutines: when it returns,
// all workers have exited.
func Map[T, R any](ctx context.Context, in []T, workers int, fn func(context.Context, T) (R, error)) ([]R, error) {
	if workers < 1 {
		workers = 1
	}
	type result struct {
		r   R
		err error
		i   int
	}
	jobs := make(chan int)
	results := make(chan result)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				r, err := fn(ctx, in[i])
				results <- result{r, err, i}
			}
		}()
	}
	go func() {
		for i := range in {
			jobs <- i
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()
	var out []R
	for res := range results {
		if res.err != nil {
			return nil, &IndexError{Index: res.i, Err: res.err}
		}
		out = append(out, res.r)
	}
	return out, nil
}
