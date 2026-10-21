package page

import "errors"

var ErrBadSize = errors.New("page: size must be >= 1")
var ErrBadPage = errors.New("page: page numbers start at 1")

type Page[T any] struct {
	Items      []T
	Number     int
	TotalPages int
	HasNext    bool
	HasPrev    bool
}

func Get[T any](items []T, size, n int) (Page[T], error) {
	if size < 1 {
		return Page[T]{}, ErrBadSize
	}
	if n < 1 {
		return Page[T]{}, ErrBadPage
	}
	total := (len(items) + size - 1) / size
	start := (n - 1) * size
	if start > len(items) {
		start = len(items)
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return Page[T]{Items: append([]T(nil), items[start:end]...), Number: n, TotalPages: total, HasNext: n < total, HasPrev: n > 1}, nil
}

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
