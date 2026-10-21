package query

import (
	"testing"

	"example.com/tasker/internal/model"
)

func TestApply(t *testing.T) {
	ts := []model.Task{{ID: 2, Priority: 1}, {ID: 1, Priority: 5, Done: true}, {ID: 3, Priority: 1}}
	got, _ := Apply(ts, Options{SortBy: "priority"})
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("%+v", got)
	}
	if _, err := Apply(ts, Options{SortBy: "x"}); err == nil {
		t.Fatal("bad sort accepted")
	}
}
