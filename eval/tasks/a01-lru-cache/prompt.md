Implement a generic LRU cache in package `lru` (file `lru/lru.go`; the stub already exists).

API (keep these exact signatures):
- `func New[K comparable, V any](capacity int) *Cache[K, V]` — capacity < 1 must panic.
- `func (c *Cache[K, V]) Get(key K) (V, bool)` — returns the value and marks the key as most recently used.
- `func (c *Cache[K, V]) Put(key K, value V)` — inserts or updates; updating also marks the key most recently used. When inserting a new key into a full cache, evict the least recently used key first.
- `func (c *Cache[K, V]) Len() int`
- `func (c *Cache[K, V]) Keys() []K` — keys ordered from most to least recently used.
- `func (c *Cache[K, V]) Remove(key K) bool` — reports whether the key was present.
- `OnEvict func(key K, value V)` exported field: if non-nil, called when a key is evicted because of capacity (not on Remove or update).

All operations must be O(1) except Keys. Add tests of your own in `lru/lru_test.go`.
