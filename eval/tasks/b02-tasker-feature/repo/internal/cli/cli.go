// Package cli implements the tasker commands.
package cli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"example.com/tasker/internal/model"
	"example.com/tasker/internal/query"
	"example.com/tasker/internal/store"
)

// Env carries the process environment so commands are testable.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	File   string           // data file; "" means tasks.json
	Now    func() time.Time // clock
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

// Run executes one command and returns the process exit code:
// 0 success, 1 runtime error, 2 usage error.
func Run(args []string, env Env) int {
	if env.File == "" {
		env.File = "tasks.json"
	}
	if env.Now == nil {
		env.Now = time.Now
	}
	err := dispatch(args, env)
	if err == nil {
		return 0
	}
	fmt.Fprintln(env.Stderr, "tasker:", err)
	var ue usageError
	if errors.As(err, &ue) {
		return 2
	}
	return 1
}

func dispatch(args []string, env Env) error {
	if len(args) == 0 {
		return usagef("missing command (add, list, done, rm, edit)")
	}
	s, err := store.Open(env.File)
	if err != nil {
		return err
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "add":
		return cmdAdd(s, rest, env)
	case "list":
		return cmdList(s, rest, env)
	case "done":
		return cmdDone(s, rest, env)
	case "rm":
		return cmdRm(s, rest, env)
	case "edit":
		return cmdEdit(s, rest, env)
	}
	return usagef("unknown command %q", cmd)
}

// flagValue returns the value for a flag at args[i], supporting "-p 3" and "-p=3".
func flagValue(args []string, i int) (name, value string, next int, err error) {
	a := args[i]
	if j := strings.IndexByte(a, '='); j >= 0 {
		return a[:j], a[j+1:], i + 1, nil
	}
	if i+1 >= len(args) {
		return a, "", i, usagef("flag %s needs a value", a)
	}
	return a, args[i+1], i + 2, nil
}

func parsePriority(v string) (int, error) {
	p, err := strconv.Atoi(v)
	if err != nil {
		return 0, usagef("bad priority %q", v)
	}
	return p, nil
}

func parseID(args []string) (int, error) {
	if len(args) != 1 {
		return 0, usagef("expected exactly one task id")
	}
	id, err := strconv.Atoi(args[0])
	if err != nil || id < 1 {
		return 0, usagef("bad task id %q", args[0])
	}
	return id, nil
}

func cmdAdd(s *store.Store, args []string, env Env) error {
	t := model.Task{Priority: model.DefaultPriority, Created: env.Now().UTC()}
	var words []string
	for i := 0; i < len(args); {
		a := args[i]
		switch {
		case a == "-p" || strings.HasPrefix(a, "-p="):
			_, v, next, err := flagValue(args, i)
			if err != nil {
				return err
			}
			if t.Priority, err = parsePriority(v); err != nil {
				return err
			}
			i = next
		case strings.HasPrefix(a, "-") && a != "-":
			return usagef("unknown flag %s", a)
		default:
			words = append(words, a)
			i++
		}
	}
	t.Title = strings.Join(words, " ")
	added, err := s.Add(t)
	if err != nil {
		return usagef("%v", err)
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "added #%d\n", added.ID)
	return nil
}

func cmdList(s *store.Store, args []string, env Env) error {
	var o query.Options
	for i := 0; i < len(args); {
		a := args[i]
		switch {
		case a == "--all":
			o.IncludeDone = true
			i++
		case a == "--sort" || strings.HasPrefix(a, "--sort="):
			_, v, next, err := flagValue(args, i)
			if err != nil {
				return err
			}
			o.SortBy = v
			i = next
		default:
			return usagef("unexpected argument %s", a)
		}
	}
	tasks, err := query.Apply(s.All(), o)
	if err != nil {
		return usagef("%v", err)
	}
	for _, t := range tasks {
		fmt.Fprintln(env.Stdout, FormatTask(t))
	}
	return nil
}

func cmdDone(s *store.Store, args []string, env Env) error {
	id, err := parseID(args)
	if err != nil {
		return err
	}
	t, err := s.Get(id)
	if err != nil {
		return err
	}
	t.Done = true
	if err := s.Update(t); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "done #%d\n", id)
	return nil
}

func cmdRm(s *store.Store, args []string, env Env) error {
	id, err := parseID(args)
	if err != nil {
		return err
	}
	if err := s.Delete(id); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "removed #%d\n", id)
	return nil
}

func cmdEdit(s *store.Store, args []string, env Env) error {
	if len(args) == 0 {
		return usagef("expected a task id")
	}
	id, err := parseID(args[:1])
	if err != nil {
		return err
	}
	t, err := s.Get(id)
	if err != nil {
		return err
	}
	rest := args[1:]
	for i := 0; i < len(rest); {
		a := rest[i]
		switch {
		case a == "-p" || strings.HasPrefix(a, "-p="):
			_, v, next, err := flagValue(rest, i)
			if err != nil {
				return err
			}
			if t.Priority, err = parsePriority(v); err != nil {
				return err
			}
			i = next
		case a == "--title" || strings.HasPrefix(a, "--title="):
			_, v, next, err := flagValue(rest, i)
			if err != nil {
				return err
			}
			t.Title = v
			i = next
		default:
			return usagef("unexpected argument %s", a)
		}
	}
	if err := s.Update(t); err != nil {
		if errors.Is(err, model.ErrEmptyTitle) || errors.Is(err, model.ErrBadPriority) {
			return usagef("%v", err)
		}
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "updated #%d\n", id)
	return nil
}
