// Package query filters and sorts tasks.
package query

import (
	"fmt"
	"sort"

	"example.com/tasker/internal/model"
)

// Options selects and orders tasks.
type Options struct {
	IncludeDone bool
	SortBy      string // "id" (default) or "priority"
}

// Apply returns the matching tasks in order.
func Apply(tasks []model.Task, o Options) ([]model.Task, error) {
	var out []model.Task
	for _, t := range tasks {
		if t.Done && !o.IncludeDone {
			continue
		}
		out = append(out, t)
	}
	switch o.SortBy {
	case "", "id":
		sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	case "priority":
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Priority != out[j].Priority {
				return out[i].Priority < out[j].Priority
			}
			return out[i].ID < out[j].ID
		})
	default:
		return nil, fmt.Errorf("unknown sort key %q", o.SortBy)
	}
	return out, nil
}
