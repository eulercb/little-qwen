package lru

import "container/list"

type entry[K comparable, V any] struct {
	k K
	v V
}

type Cache[K comparable, V any] struct {
	cap     int
	ll      *list.List
	m       map[K]*list.Element
	OnEvict func(key K, value V)
}

func New[K comparable, V any](capacity int) *Cache[K, V] {
	if capacity < 1 {
		panic("lru: capacity < 1")
	}
	return &Cache[K, V]{cap: capacity, ll: list.New(), m: map[K]*list.Element{}}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	if e, ok := c.m[key]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(*entry[K, V]).v, true
	}
	var z V
	return z, false
}

func (c *Cache[K, V]) Put(key K, value V) {
	if e, ok := c.m[key]; ok {
		e.Value.(*entry[K, V]).v = value
		c.ll.MoveToFront(e)
		return
	}
	if c.ll.Len() >= c.cap {
		b := c.ll.Back()
		en := b.Value.(*entry[K, V])
		c.ll.Remove(b)
		delete(c.m, en.k)
		if c.OnEvict != nil {
			c.OnEvict(en.k, en.v)
		}
	}
	c.m[key] = c.ll.PushFront(&entry[K, V]{key, value})
}

func (c *Cache[K, V]) Len() int { return c.ll.Len() }

func (c *Cache[K, V]) Keys() []K {
	out := make([]K, 0, c.ll.Len())
	for e := c.ll.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*entry[K, V]).k)
	}
	return out
}

func (c *Cache[K, V]) Remove(key K) bool {
	if e, ok := c.m[key]; ok {
		c.ll.Remove(e)
		delete(c.m, key)
		return true
	}
	return false
}
