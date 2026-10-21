// Package model defines the task type.
package model

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Task struct {
	ID       int        `json:"id"`
	Title    string     `json:"title"`
	Priority int        `json:"priority"`
	Done     bool       `json:"done"`
	Created  time.Time  `json:"created"`
	Tags     []string   `json:"tags,omitempty"`
	Due      *time.Time `json:"due,omitempty"`
}

var (
	ErrEmptyTitle  = errors.New("title must not be empty")
	ErrBadPriority = errors.New("priority must be between 1 and 5")
	ErrBadTag      = errors.New("bad tag")
)

const DefaultPriority = 3

var tagRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func NormalizeTags(tags []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if !tagRe.MatchString(t) {
			return nil, ErrBadTag
		}
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (t *Task) Validate() error {
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" {
		return ErrEmptyTitle
	}
	if t.Priority < 1 || t.Priority > 5 {
		return ErrBadPriority
	}
	tags, err := NormalizeTags(t.Tags)
	if err != nil {
		return err
	}
	t.Tags = tags
	return nil
}

func (t Task) HasTag(tag string) bool {
	for _, x := range t.Tags {
		if x == tag {
			return true
		}
	}
	return false
}
