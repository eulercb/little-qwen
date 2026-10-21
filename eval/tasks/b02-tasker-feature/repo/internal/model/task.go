// Package model defines the task type.
package model

import (
	"errors"
	"strings"
	"time"
)

// Task is one tracked item.
type Task struct {
	ID       int       `json:"id"`
	Title    string    `json:"title"`
	Priority int       `json:"priority"`
	Done     bool      `json:"done"`
	Created  time.Time `json:"created"`
}

// Validation errors.
var (
	ErrEmptyTitle  = errors.New("title must not be empty")
	ErrBadPriority = errors.New("priority must be between 1 and 5")
)

// DefaultPriority is used when none is given.
const DefaultPriority = 3

// Validate checks the task fields.
func (t *Task) Validate() error {
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" {
		return ErrEmptyTitle
	}
	if t.Priority < 1 || t.Priority > 5 {
		return ErrBadPriority
	}
	return nil
}
