// Package page splits result lists into pages.
package page

import "errors"

// ErrBadSize is returned for page sizes < 1.
var ErrBadSize = errors.New("page: size must be >= 1")

// ErrBadPage is returned for page numbers < 1.
var ErrBadPage = errors.New("page: page numbers start at 1")

// Page is one page of results.
type Page[T any] struct {
	Items      []T
	Number     int  // 1-based page number requested
	TotalPages int  // number of non-empty pages (0 for an empty input)
	HasNext    bool // true if a later page has items
	HasPrev    bool // true if Number > 1
}

// Get returns page number n (1-based) of items, with size items per page.
// Pages past the end are returned with no Items (HasPrev still reflects n).
// The returned Items slice must not alias the input (callers may modify it).
func Get[T any](items []T, size, n int) (Page[T], error) {
	if size < 1 {
		return Page[T]{}, ErrBadSize
	}
	if n < 1 {
		return Page[T]{}, ErrBadPage
	}
	total := len(items)/size + 1
	if len(items) == 0 {
		total = 0
	}
	start := (n - 1) * size
	end := start + size
	if end > len(items) {
		end = len(items) - 1
	}
	p := Page[T]{
		Items:      items[start:end],
		Number:     n,
		TotalPages: total,
		HasNext:    n < total,
		HasPrev:    n > 1,
	}
	return p, nil
}

// All splits items into consecutive pages.
func All[T any](items []T, size int) ([][]T, error) {
	if size < 1 {
		return nil, ErrBadSize
	}
	var out [][]T
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out, nil
}
