package eval

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTasksBaseAndSuffix(t *testing.T) {
	tasks, err := LoadTasks("../../eval/tasks", []string{"d01s-caca-palavras-single"})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("load: %v %d", err, len(tasks))
	}
	d := tasks[0]
	if filepath.Base(d.Dir) != "d01-caca-palavras" || !filepath.IsAbs(d.Dir) {
		t.Fatalf("Dir should be the base task's absolute dir: %s", d.Dir)
	}
	if len(d.Sessions) != 0 || !strings.Contains(d.Prompt, "# This session\nImplement the WHOLE application") || len(d.Prompt) < 40000 {
		t.Fatalf("prompt: sessions=%d len=%d", len(d.Sessions), len(d.Prompt))
	}
}
