// Package store persists tasks in a JSON file.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"example.com/tasker/internal/model"
)

// Version is the current file schema version.
const Version = 2

// ErrNotFound is returned for unknown task IDs.
var ErrNotFound = errors.New("task not found")

type fileFormat struct {
	Version int          `json:"version"`
	NextID  int          `json:"next_id"`
	Tasks   []model.Task `json:"tasks"`
}

// Store is an in-memory view of the task file.
type Store struct {
	path   string
	nextID int
	tasks  map[int]*model.Task
}

// Open loads the file at path. A missing file yields an empty store.
func Open(path string) (*Store, error) {
	s := &Store{path: path, nextID: 1, tasks: map[int]*model.Task{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("store: parse %s: %w", path, err)
	}
	if f.Version > Version {
		return nil, fmt.Errorf("store: file version %d is newer than supported %d", f.Version, Version)
	}
	for i := range f.Tasks {
		t := f.Tasks[i]
		s.tasks[t.ID] = &t
		if t.ID >= s.nextID {
			s.nextID = t.ID + 1
		}
	}
	if f.NextID > s.nextID {
		s.nextID = f.NextID
	}
	return s, nil
}

// Save writes the store atomically.
func (s *Store) Save() error {
	f := fileFormat{Version: Version, NextID: s.nextID, Tasks: s.All()}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Add validates and inserts a task, assigning its ID.
func (s *Store) Add(t model.Task) (model.Task, error) {
	if err := t.Validate(); err != nil {
		return model.Task{}, err
	}
	t.ID = s.nextID
	s.nextID++
	s.tasks[t.ID] = &t
	return t, nil
}

// Get returns a copy of a task.
func (s *Store) Get(id int) (model.Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return model.Task{}, ErrNotFound
	}
	return *t, nil
}

// Update replaces a task after validation.
func (s *Store) Update(t model.Task) error {
	if _, ok := s.tasks[t.ID]; !ok {
		return ErrNotFound
	}
	if err := t.Validate(); err != nil {
		return err
	}
	s.tasks[t.ID] = &t
	return nil
}

// Delete removes a task.
func (s *Store) Delete(id int) error {
	if _, ok := s.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}

// All returns copies of all tasks ordered by ID.
func (s *Store) All() []model.Task {
	out := make([]model.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
