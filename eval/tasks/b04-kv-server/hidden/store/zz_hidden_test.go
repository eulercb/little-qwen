package store

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type clk struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clk) now() time.Time      { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clk) add(d time.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

func newS() (*Store, *clk) {
	c := &clk{t: time.Unix(1_700_000_000, 0)}
	return New(c.now), c
}

func TestHiddenExpiry(t *testing.T) {
	s, c := newS()
	s.Set("a", "1", 10*time.Second)
	s.Set("b", "2", 0)
	if s.TTL("a") != 10*time.Second || s.TTL("b") != -1 || s.TTL("zz") != -2 {
		t.Fatalf("ttl %v %v %v", s.TTL("a"), s.TTL("b"), s.TTL("zz"))
	}
	c.add(10 * time.Second)
	if _, ok := s.Get("a"); ok || s.Exists("a") || s.Len() != 1 {
		t.Fatal("a should be expired exactly at deadline")
	}
	s.Set("c", "x", time.Second)
	s.Set("c", "y", 0)
	c.add(5 * time.Second)
	if v, ok := s.Get("c"); !ok || v != "y" {
		t.Fatal("Set must reset ttl")
	}
}

func TestHiddenExpirePersist(t *testing.T) {
	s, c := newS()
	if s.Expire("nope", time.Second) {
		t.Fatal("expire missing")
	}
	s.Set("k", "v", 0)
	if s.Persist("k") {
		t.Fatal("persist without ttl must be false")
	}
	s.Expire("k", 3*time.Second)
	if !s.Persist("k") || s.TTL("k") != -1 {
		t.Fatal("persist")
	}
	s.Expire("k", 2*time.Second)
	c.add(2 * time.Second)
	if s.Persist("k") {
		t.Fatal("persist on expired key")
	}
	s.Set("d", "v", 0)
	if !s.Expire("d", 0) || s.Exists("d") {
		t.Fatal("expire <= 0 deletes")
	}
}

func TestHiddenDelIncr(t *testing.T) {
	s, c := newS()
	s.Set("a", "1", 0)
	s.Set("b", "x", time.Second)
	c.add(time.Second)
	if n := s.Del("a", "b", "c", "a"); n != 1 {
		t.Fatalf("del %d", n)
	}
	if n, err := s.Incr("n", 5); err != nil || n != 5 {
		t.Fatal(n, err)
	}
	s.Expire("n", 100*time.Second)
	if n, _ := s.Incr("n", -7); n != -2 || s.TTL("n") != 100*time.Second {
		t.Fatal("incr keeps ttl")
	}
	s.Set("s", "abc", 0)
	if _, err := s.Incr("s", 1); !errors.Is(err, ErrNotInteger) {
		t.Fatal("not integer")
	}
	s.Set("big", "9223372036854775807x", 0)
	if _, err := s.Incr("big", 1); !errors.Is(err, ErrNotInteger) {
		t.Fatal("garbage int")
	}
}

func TestHiddenKeysGlob(t *testing.T) {
	s, c := newS()
	for _, k := range []string{"user:1", "user:2", "user:10", "users", "a/b/c", "x*y", "cat", "cot", "cut", "c[t"} {
		s.Set(k, "v", 0)
	}
	s.Set("user:3", "v", time.Second)
	c.add(time.Second)
	cases := map[string][]string{
		"user:*":   {"user:1", "user:10", "user:2"},
		"user:?":   {"user:1", "user:2"},
		"a*":       {"a/b/c"},
		"*/*":      {"a/b/c"},
		"c[ao]t":   {"cat", "cot"},
		"c[^ao]t":  {"c[t", "cut"},
		"c[!a-o]t": {"c[t", "cut"},
		"x\\*y":    {"x*y"},
		"c\\[t":    {"c[t"},
		"nomatch*": nil,
		"*":        {"a/b/c", "c[t", "cat", "cot", "cut", "user:1", "user:10", "user:2", "users", "x*y"},
	}
	for p, want := range cases {
		got := s.Keys(p)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Keys(%q) = %v want %v", p, got, want)
		}
	}
}

func TestHiddenSnapshot(t *testing.T) {
	s, c := newS()
	s.Set("b", "2", 0)
	s.Set("a", "1", 90*time.Second)
	s.Set("gone", "x", time.Second)
	c.add(time.Second)
	var buf bytes.Buffer
	if err := s.Save(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"version":1`) || strings.Contains(out, "gone") || strings.Index(out, `"a"`) > strings.Index(out, `"b"`) {
		t.Fatalf("snapshot %s", out)
	}
	s2, c2 := newS()
	c2.t = c.t
	s2.Set("old", "x", 0)
	if err := s2.Load(strings.NewReader(out)); err != nil {
		t.Fatal(err)
	}
	if s2.Exists("old") || s2.Len() != 2 || s2.TTL("a") != 89*time.Second {
		t.Fatalf("load: len=%d ttl=%v", s2.Len(), s2.TTL("a"))
	}
	c2.add(100 * time.Second)
	s3, c3 := newS()
	c3.t = c2.t
	s3.Load(strings.NewReader(out))
	if s3.Exists("a") || !s3.Exists("b") {
		t.Fatal("expired entries must be dropped on load")
	}
	if err := s3.Load(strings.NewReader(`{"version":2,"entries":[]}`)); err == nil {
		t.Fatal("bad version accepted")
	}
}

func TestHiddenConcurrent(t *testing.T) {
	s, _ := newS()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				s.Incr("n", 1)
				s.Keys("*")
				s.Set("k", "v", time.Minute)
				s.TTL("k")
			}
		}()
	}
	wg.Wait()
	if v, _ := s.Get("n"); v != "1600" {
		t.Fatal(v)
	}
}
