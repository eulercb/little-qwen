package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type harness struct {
	t    *testing.T
	file string
	now  time.Time
}

func newHarness(t *testing.T) *harness {
	return &harness{t: t, file: filepath.Join(t.TempDir(), "tasks.json"), now: time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)}
}

func (h *harness) run(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := Run(args, Env{Stdout: &out, Stderr: &errb, File: h.file, Now: func() time.Time { return h.now }})
	return out.String(), errb.String(), code
}

func TestAddListDone(t *testing.T) {
	h := newHarness(t)
	if out, _, code := h.run("add", "-p", "1", "write", "report"); code != 0 || out != "added #1\n" {
		t.Fatalf("%q %d", out, code)
	}
	h.run("add", "buy milk")
	h.run("done", "1")
	out, _, _ := h.run("list")
	if out != "#2 [ ] P3 buy milk\n" {
		t.Fatalf("list: %q", out)
	}
	out, _, _ = h.run("list", "--all", "--sort=priority")
	if !strings.HasPrefix(out, "#1 [x] P1 write report\n") {
		t.Fatalf("list --all: %q", out)
	}
}

func TestUsageErrors(t *testing.T) {
	h := newHarness(t)
	if _, _, code := h.run("frob"); code != 2 {
		t.Fatal("unknown command")
	}
	if _, _, code := h.run("add", "-p", "9", "x"); code != 2 {
		t.Fatal("bad priority")
	}
	if _, _, code := h.run("done", "42"); code != 1 {
		t.Fatal("missing task is a runtime error")
	}
}
