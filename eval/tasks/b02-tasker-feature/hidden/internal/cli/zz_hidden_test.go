package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/tasker/internal/model"
	"example.com/tasker/internal/store"
)

func TestHiddenAddWithTagsAndDue(t *testing.T) {
	h := newHarness(t)
	if _, e, code := h.run("add", "-p", "2", "--tag", "Home", "--tag=money", "--due", "2026-04-15", "file", "taxes"); code != 0 {
		t.Fatalf("add: %d %s", code, e)
	}
	h.run("add", "--tag", "home", "--tag", "home", "water plants")
	out, _, _ := h.run("list")
	want := "#1 [ ] P2 file taxes (due 2026-04-15) [home,money]\n#2 [ ] P3 water plants [home]\n"
	if out != want {
		t.Fatalf("list:\n%q\nwant\n%q", out, want)
	}
}

func TestHiddenBadInputs(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"add", "--due", "2026-13-01", "x"},
		{"add", "--due", "tomorrow", "x"},
		{"add", "--tag", "bad tag", "x"},
		{"add", "--tag", "-lead", "x"},
		{"add", "--tag", "", "x"},
	} {
		if _, _, code := h.run(args...); code != 2 {
			t.Errorf("%v: exit %d want 2", args, code)
		}
	}
	h.run("add", "ok")
	if _, _, code := h.run("edit", "1", "--tag", "no!"); code != 2 {
		t.Errorf("edit bad tag: exit %d", code)
	}
	if !errorsIsBadTag() {
		t.Error("model.ErrBadTag not returned by Validate")
	}
}

func errorsIsBadTag() bool {
	tk := model.Task{Title: "x", Priority: 3, Tags: []string{"a b"}}
	return tk.Validate() == model.ErrBadTag
}

func TestHiddenEditTagsAndDue(t *testing.T) {
	h := newHarness(t)
	h.run("add", "--tag", "a", "--due", "2026-03-01", "task")
	if _, e, code := h.run("edit", "1", "--tag", "B", "--untag", "a", "--untag", "zzz"); code != 0 {
		t.Fatalf("edit: %s", e)
	}
	out, _, _ := h.run("list")
	if out != "#1 [ ] P3 task (due 2026-03-01) [b]\n" {
		t.Fatalf("%q", out)
	}
	h.run("edit", "1", "--due", "none")
	out, _, _ = h.run("list")
	if out != "#1 [ ] P3 task [b]\n" {
		t.Fatalf("%q", out)
	}
	h.run("edit", "1", "--due=2026-05-05")
	out, _, _ = h.run("list")
	if out != "#1 [ ] P3 task (due 2026-05-05) [b]\n" {
		t.Fatalf("%q", out)
	}
}

func TestHiddenListFilters(t *testing.T) {
	h := newHarness(t)                                                           // today is 2026-03-10
	h.run("add", "--tag", "work", "--due", "2026-03-09", "a")                    // 1 overdue
	h.run("add", "--tag", "work", "--tag", "urgent", "--due", "2026-03-10", "b") // 2 due today, not overdue
	h.run("add", "--tag", "home", "c")                                           // 3
	h.run("add", "--tag", "work", "--due", "2026-01-01", "d")                    // 4 overdue but done
	h.run("done", "4")
	h.run("add", "--due", "2026-02-01", "e") // 5 overdue
	out, _, _ := h.run("list", "--tag", "work")
	if out != "#1 [ ] P3 a (due 2026-03-09) [work]\n#2 [ ] P3 b (due 2026-03-10) [urgent,work]\n" {
		t.Fatalf("tag filter %q", out)
	}
	out, _, _ = h.run("list", "--tag", "work", "--tag", "URGENT")
	if out != "#2 [ ] P3 b (due 2026-03-10) [urgent,work]\n" {
		t.Fatalf("AND filter %q", out)
	}
	out, _, _ = h.run("list", "--overdue", "--all")
	if out != "#1 [ ] P3 a (due 2026-03-09) [work]\n#5 [ ] P3 e (due 2026-02-01)\n" {
		t.Fatalf("overdue %q", out)
	}
	out, _, _ = h.run("list", "--sort", "due")
	ids := idsOf(out)
	if ids != "5,1,2,3" {
		t.Fatalf("sort due: %s (%q)", ids, out)
	}
	out, _, _ = h.run("list", "--all", "--sort=due")
	if idsOf(out) != "4,5,1,2,3" {
		t.Fatalf("sort due all: %q", out)
	}
}

func idsOf(out string) string {
	var ids []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if l == "" {
			continue
		}
		f := strings.Fields(l)[0]
		ids = append(ids, strings.TrimPrefix(f, "#"))
	}
	return strings.Join(ids, ",")
}

func TestHiddenTagsCommand(t *testing.T) {
	h := newHarness(t)
	h.run("add", "--tag", "work", "a")
	h.run("add", "--tag", "work", "--tag", "home", "b")
	h.run("add", "--tag", "zeta", "c")
	h.run("done", "3")
	out, _, code := h.run("tags")
	if code != 0 || out != "home 1\nwork 2\n" {
		t.Fatalf("tags: %d %q", code, out)
	}
	_, e, code := h.run()
	if code != 2 || !strings.Contains(e, "tags") {
		t.Fatalf("usage message: %q", e)
	}
}

func TestHiddenSchemaV1Migration(t *testing.T) {
	h := newHarness(t)
	v1 := `{"version":1,"next_id":3,"tasks":[{"id":1,"title":"old","priority":2,"done":false,"created":"2025-01-01T00:00:00Z"}]}`
	if err := os.WriteFile(h.file, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}
	out, e, code := h.run("list")
	if code != 0 || out != "#1 [ ] P2 old\n" {
		t.Fatalf("v1 load: %d %q %s", code, out, e)
	}
	h.run("edit", "1", "--tag", "legacy", "--due", "2026-12-31")
	data, _ := os.ReadFile(h.file)
	var f map[string]any
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f["version"] != float64(2) || store.Version != 2 {
		t.Fatalf("saved version %v", f["version"])
	}
	tasks := f["tasks"].([]any)
	tk := tasks[0].(map[string]any)
	if tk["due"] == nil || tk["tags"] == nil {
		t.Fatalf("fields not saved: %v", tk)
	}
	due, _ := time.Parse(time.RFC3339, tk["due"].(string))
	if !due.Equal(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("due stored as %v", tk["due"])
	}
	if out, _, _ := h.run("add", "new"); out != "added #3\n" {
		t.Fatalf("next id: %q", out)
	}
	// Tasks without tags/due must omit the fields.
	data, _ = os.ReadFile(h.file)
	if strings.Count(string(data), `"tags"`) != 1 || strings.Count(string(data), `"due"`) != 1 {
		t.Fatalf("omitempty: %s", data)
	}
}
