package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"sync"
	"time"
)

type Clock func() time.Time

var ErrNotInteger = errors.New("value is not an integer")

type entry struct {
	v   string
	exp time.Time
}

type Store struct {
	mu    sync.Mutex
	clock Clock
	m     map[string]entry
}

func New(clock Clock) *Store {
	if clock == nil {
		clock = time.Now
	}
	return &Store{clock: clock, m: map[string]entry{}}
}

func (s *Store) live(k string, now time.Time) (entry, bool) {
	e, ok := s.m[k]
	if !ok {
		return entry{}, false
	}
	if !e.exp.IsZero() && !now.Before(e.exp) {
		delete(s.m, k)
		return entry{}, false
	}
	return e, true
}

func (s *Store) Set(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := entry{v: value}
	if ttl > 0 {
		e.exp = s.clock().Add(ttl)
	}
	s.m[key] = e
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.live(key, s.clock())
	return e.v, ok
}

func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	n := 0
	for _, k := range keys {
		if _, ok := s.live(k, now); ok {
			delete(s.m, k)
			n++
		}
	}
	return n
}

func (s *Store) Exists(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.live(key, s.clock())
	return ok
}

func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	e, ok := s.live(key, now)
	if !ok {
		return false
	}
	if ttl <= 0 {
		delete(s.m, key)
		return true
	}
	e.exp = now.Add(ttl)
	s.m[key] = e
	return true
}

func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.live(key, s.clock())
	if !ok || e.exp.IsZero() {
		return false
	}
	e.exp = time.Time{}
	s.m[key] = e
	return true
}

func (s *Store) TTL(key string) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	e, ok := s.live(key, now)
	if !ok {
		return -2
	}
	if e.exp.IsZero() {
		return -1
	}
	return e.exp.Sub(now)
}

func (s *Store) Incr(key string, by int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.live(key, s.clock())
	var n int64
	if ok {
		var err error
		n, err = strconv.ParseInt(e.v, 10, 64)
		if err != nil {
			return 0, ErrNotInteger
		}
	}
	n += by
	e.v = strconv.FormatInt(n, 10)
	s.m[key] = e
	return n, nil
}

func (s *Store) Keys(pattern string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	var out []string
	for k := range s.m {
		if _, ok := s.live(k, now); ok && Match(pattern, k) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	n := 0
	for k := range s.m {
		if _, ok := s.live(k, now); ok {
			n++
		}
	}
	return n
}

type snapEntry struct {
	K   string `json:"k"`
	V   string `json:"v"`
	Exp int64  `json:"exp"`
}

type snapshot struct {
	Version int         `json:"version"`
	Entries []snapEntry `json:"entries"`
}

func (s *Store) Save(w io.Writer) error {
	s.mu.Lock()
	now := s.clock()
	snap := snapshot{Version: 1, Entries: []snapEntry{}}
	for k := range s.m {
		if e, ok := s.live(k, now); ok {
			var exp int64
			if !e.exp.IsZero() {
				exp = e.exp.UnixMilli()
			}
			snap.Entries = append(snap.Entries, snapEntry{k, e.v, exp})
		}
	}
	s.mu.Unlock()
	sort.Slice(snap.Entries, func(i, j int) bool { return snap.Entries[i].K < snap.Entries[j].K })
	return json.NewEncoder(w).Encode(snap)
}

func (s *Store) Load(r io.Reader) error {
	var snap snapshot
	if err := json.NewDecoder(r).Decode(&snap); err != nil {
		return err
	}
	if snap.Version != 1 {
		return fmt.Errorf("store: unsupported snapshot version %d", snap.Version)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock()
	m := map[string]entry{}
	for _, e := range snap.Entries {
		en := entry{v: e.V}
		if e.Exp != 0 {
			en.exp = time.UnixMilli(e.Exp)
			if !now.Before(en.exp) {
				continue
			}
		}
		m[e.K] = en
	}
	s.m = m
	return nil
}

// Match reports whether name matches the glob pattern.
func Match(pattern, name string) bool {
	p, n := []rune(pattern), []rune(name)
	var rec func(pi, ni int) bool
	rec = func(pi, ni int) bool {
		for pi < len(p) {
			switch p[pi] {
			case '*':
				for k := ni; k <= len(n); k++ {
					if rec(pi+1, k) {
						return true
					}
				}
				return false
			case '?':
				if ni >= len(n) {
					return false
				}
				pi++
				ni++
			case '[':
				if ni >= len(n) {
					return false
				}
				j := pi + 1
				neg := false
				if j < len(p) && (p[j] == '^' || p[j] == '!') {
					neg = true
					j++
				}
				matched := false
				first := true
				for j < len(p) && (p[j] != ']' || first) {
					first = false
					lo := p[j]
					if lo == '\\' && j+1 < len(p) {
						j++
						lo = p[j]
					}
					hi := lo
					if j+2 < len(p) && p[j+1] == '-' && p[j+2] != ']' {
						hi = p[j+2]
						j += 2
					}
					if n[ni] >= lo && n[ni] <= hi {
						matched = true
					}
					j++
				}
				if j >= len(p) {
					return false
				}
				if matched == neg {
					return false
				}
				pi = j + 1
				ni++
			case '\\':
				if pi+1 < len(p) {
					pi++
				}
				fallthrough
			default:
				if ni >= len(n) || n[ni] != p[pi] {
					return false
				}
				pi++
				ni++
			}
		}
		return ni == len(n)
	}
	return rec(0, 0)
}
