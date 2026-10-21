package query

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/tasker/internal/model"
)

type Options struct {
	IncludeDone bool
	SortBy      string
	Tags        []string
	Overdue     bool
	Today       time.Time
}

func Apply(tasks []model.Task, o Options) ([]model.Task, error) {
	var out []model.Task
	for _, t := range tasks {
		if t.Done && !o.IncludeDone {
			continue
		}
		ok := true
		for _, tag := range o.Tags {
			if !t.HasTag(strings.ToLower(strings.TrimSpace(tag))) {
				ok = false
			}
		}
		if o.Overdue && (t.Done || t.Due == nil || !t.Due.Before(o.Today)) {
			ok = false
		}
		if ok {
			out = append(out, t)
		}
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
	case "due":
		sort.SliceStable(out, func(i, j int) bool {
			a, b := out[i].Due, out[j].Due
			switch {
			case a != nil && b != nil && !a.Equal(*b):
				return a.Before(*b)
			case a != nil && b == nil:
				return true
			case a == nil && b != nil:
				return false
			}
			return out[i].ID < out[j].ID
		})
	default:
		return nil, fmt.Errorf("unknown sort key %q", o.SortBy)
	}
	return out, nil
}
