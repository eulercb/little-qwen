// Package ctxmgr holds the context policy helpers: token estimates, tool
// output capping with spill files, and the deterministic state snapshot used
// at compaction.
package ctxmgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eulercb/little-qwen/internal/tools"
)

// CharsPerToken is a conservative estimate for code-heavy text with the
// Qwen tokenizer. The server's usage.prompt_tokens is the source of truth;
// this is only used to size tool output before it is sent.
const CharsPerToken = 3.3

func EstTokens(s string) int { return int(float64(len(s))/CharsPerToken) + 1 }

// Cap trims out to capTokens. Oversized output is written in full to
// <root>/.qh/spill/<name>.txt and replaced with head + tail + a pointer.
func Cap(root, name, out string, capTokens int) (string, bool) {
	limit := int(float64(capTokens) * CharsPerToken)
	if len(out) <= limit {
		return out, false
	}
	dir := filepath.Join(root, ".qh", "spill")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, name+".txt")
	_ = os.WriteFile(path, []byte(out), 0o644)
	rel, _ := filepath.Rel(root, path)
	lines := strings.Count(out, "\n") + 1
	headN := limit * 55 / 100
	tailN := limit * 35 / 100
	head := out[:headN]
	if i := strings.LastIndexByte(head, '\n'); i > headN/2 {
		head = head[:i+1]
	}
	tail := out[len(out)-tailN:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 && i < tailN/2 {
		tail = tail[i+1:]
	}
	omitted := strings.Count(out[len(head):len(out)-len(tail)], "\n")
	note := fmt.Sprintf("\n... [%d lines omitted: output was %d lines / ~%d tokens, over the %d-token inline cap. Full output saved to %s - use read_file with start_line/end_line or grep on it to see more.] ...\n",
		omitted, lines, EstTokens(out), capTokens, rel)
	return head + note + tail, true
}

// StateSnapshot builds the deterministic part of a compaction checkpoint
// from git and the plan file.
func StateSnapshot(ctx context.Context, root string) string {
	var b strings.Builder
	run := func(title, cmd string) {
		out, _, _ := tools.Shell(ctx, root, nil, cmd, 30*time.Second)
		out = strings.TrimSpace(out)
		if out == "" {
			out = "(none)"
		}
		if len(out) > 3000 {
			out = out[:3000] + "\n...(truncated)"
		}
		fmt.Fprintf(&b, "### %s\n```\n%s\n```\n", title, out)
	}
	if plan, err := os.ReadFile(filepath.Join(root, ".qh", "PLAN.md")); err == nil {
		fmt.Fprintf(&b, "### Plan (.qh/PLAN.md)\n%s\n", strings.TrimSpace(string(plan)))
	}
	run("Files changed since start (git status --short)", "git status --short 2>/dev/null | grep -v '^?? .qh/' | head -60")
	run("Diff size (git diff --stat)", "git add -A -N . 2>/dev/null; git diff --stat 2>/dev/null | tail -30")
	return b.String()
}
