package lru

import "testing"

func TestBasic(t *testing.T) {
	c := New[string, int](2)
	c.Put("a", 1)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %v,%v", v, ok)
	}
}
