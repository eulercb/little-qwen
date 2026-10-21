package lru

import (
	"reflect"
	"testing"
)

func TestHiddenEvictionOrder(t *testing.T) {
	c := New[int, string](3)
	c.Put(1, "a")
	c.Put(2, "b")
	c.Put(3, "c")
	c.Get(1)
	c.Put(4, "d")
	if _, ok := c.Get(2); ok {
		t.Fatal("2 should be evicted")
	}
	if got := c.Keys(); !reflect.DeepEqual(got, []int{4, 1, 3}) {
		t.Fatalf("Keys = %v", got)
	}
}

func TestHiddenUpdateMovesToFront(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("a", 10)
	c.Put("c", 3)
	if v, ok := c.Get("a"); !ok || v != 10 {
		t.Fatalf("a = %v %v", v, ok)
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d", c.Len())
	}
}

func TestHiddenOnEvict(t *testing.T) {
	c := New[string, int](1)
	var ev []string
	c.OnEvict = func(k string, v int) { ev = append(ev, k) }
	c.Put("a", 1)
	c.Put("a", 2)
	c.Put("b", 3)
	c.Remove("b")
	c.Put("c", 4)
	if !reflect.DeepEqual(ev, []string{"a"}) {
		t.Fatalf("evicted = %v", ev)
	}
}

func TestHiddenRemove(t *testing.T) {
	c := New[int, int](2)
	c.Put(1, 1)
	if !c.Remove(1) || c.Remove(1) {
		t.Fatal("Remove semantics")
	}
	if c.Len() != 0 || len(c.Keys()) != 0 {
		t.Fatal("not empty")
	}
}

func TestHiddenPanicsOnZeroCapacity(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	New[int, int](0)
}

func TestHiddenMissingGetZero(t *testing.T) {
	c := New[string, *int](1)
	v, ok := c.Get("x")
	if ok || v != nil {
		t.Fatal("expected zero value")
	}
}
