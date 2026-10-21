package store

import (
	"path/filepath"
	"testing"

	"example.com/tasker/internal/model"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "t.json")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.Add(model.Task{Title: "a", Priority: 2})
	s.Add(model.Task{Title: "b", Priority: 3})
	s.Delete(a.ID)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	all := s2.All()
	if len(all) != 1 || all[0].Title != "b" {
		t.Fatalf("%+v", all)
	}
	c, _ := s2.Add(model.Task{Title: "c", Priority: 1})
	if c.ID != 3 {
		t.Fatalf("ids must not be reused: %d", c.ID)
	}
}
