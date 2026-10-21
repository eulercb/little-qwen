// Package api implements a small notes service.
package api

import (
	"sort"
	"sync"
)

// Note is a stored note.
type Note struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Store is a concurrency-safe in-memory note store.
type Store struct {
	mu    sync.Mutex
	next  int
	notes map[int]Note
}

// NewStore returns an empty store.
func NewStore() *Store { return &Store{next: 1, notes: map[int]Note{}} }

// Create stores a new note and returns it with its assigned ID.
func (s *Store) Create(title, body string) Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := Note{ID: s.next, Title: title, Body: body}
	s.notes[n.ID] = n
	s.next++
	return n
}

// Get returns a note by ID.
func (s *Store) Get(id int) (Note, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[id]
	return n, ok
}

// Delete removes a note, reporting whether it existed.
func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.notes[id]
	delete(s.notes, id)
	return ok
}

// List returns all notes ordered by ID.
func (s *Store) List() []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
