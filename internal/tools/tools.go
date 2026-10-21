// Package tools implements the small, flat-argument tool set the executor
// model uses. Every result starts with a one-line status ("OK ..." or
// "ERROR <class>: <hint>") so the model and the trace can classify it.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/eulercb/little-qwen/internal/llm"
)

// Result is a tool outcome before output capping.
type Result struct {
	OK         bool
	ErrorClass string // set when !OK
	Output     string // full text, including the status line
	Changed    []string
	// TestSignal is set when a run looked like a build/test and reports pass/fail.
	TestSignal string
}

// Env is the execution environment of one agent.
type Env struct {
	Root    string // workdir; file tools are confined to it
	RunEnv  []string
	MaxRun  time.Duration
	ReadMax int // max lines per read_file call
}

func ok(format string, a ...any) Result {
	return Result{OK: true, Output: fmt.Sprintf(format, a...)}
}

func fail(class, format string, a ...any) Result {
	return Result{OK: false, ErrorClass: class, Output: "ERROR " + class + ": " + fmt.Sprintf(format, a...)}
}

// Resolve maps a model-supplied path into the workdir.
func (e *Env) Resolve(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		p = "."
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Join(e.Root, p)
	}
	rel, err := filepath.Rel(e.Root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the workspace %s; use a relative path", p, e.Root)
	}
	return abs, nil
}

func (e *Env) rel(abs string) string {
	r, err := filepath.Rel(e.Root, abs)
	if err != nil {
		return abs
	}
	return r
}

// Definitions returns the OpenAI tool schemas. names selects a subset (nil = all).
func Definitions(names []string) []llm.Tool {
	all := []llm.Tool{
		def("read_file", "Read a text file with line numbers. Use start_line/end_line to read a range of a large file (max 400 lines per call).",
			props{"path": str("File path relative to the workspace root."), "start_line": integer("First line, 1-based (default 1)."), "end_line": integer("Last line, inclusive (default start_line+399).")}, "path"),
		def("write_file", "Create or overwrite a file with the full content given. Parent directories are created.",
			props{"path": str("File path relative to the workspace root."), "content": str("The complete new file content.")}, "path", "content"),
		def("edit_file", "Replace one exact occurrence of old_text with new_text in a file. old_text must match exactly once (include enough surrounding lines to be unique). Fails on 0 or >1 matches.",
			props{"path": str("File path relative to the workspace root."), "old_text": str("Exact text to find, including whitespace."), "new_text": str("Replacement text.")}, "path", "old_text", "new_text"),
		def("run", "Run a bash command in the workspace root and return exit code and combined output. Use for building, testing, git, and other shell work. Not interactive.",
			props{"command": str("The bash command line."), "timeout_s": integer("Timeout in seconds (default 120, max 600).")}, "command"),
		def("grep", "Search file contents with a regular expression (Go RE2 syntax). Returns matching lines as path:line: text.",
			props{"pattern": str("Regular expression."), "path": str("Directory or file to search (default: workspace root)."), "glob": str("Optional filename glob filter, e.g. *.go")}, "pattern"),
		def("list_dir", "List files and directories as a tree.",
			props{"path": str("Directory relative to the workspace root (default .)."), "depth": integer("Max depth (default 2, max 5).")}),
		def("update_plan", "Record progress on your plan. Call when you start or finish a step. The plan is kept in .qh/PLAN.md.",
			props{"step": str("Short step name, e.g. '1. add parser'."), "status": str("One of: todo, doing, done, blocked."), "note": str("Optional one-line note.")}, "step", "status"),
		def("delegate", "Run a read-only sub-agent in a fresh context for a 'large input, small answer' job (e.g. find where X is implemented across many files, summarise a long log). It returns a short report. Do not delegate edits.",
			props{"task": str("Self-contained instructions and the exact question to answer.")}, "task"),
		def("finish", "Declare the task complete. The harness then runs the acceptance check; if it fails you will be asked to continue.",
			props{"summary": str("2-5 sentences: what you changed and how you verified it.")}, "summary"),
	}
	if names == nil {
		return all
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var out []llm.Tool
	for _, t := range all {
		if want[t.Function.Name] {
			out = append(out, t)
		}
	}
	return out
}

type props map[string]any

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func def(name, desc string, p props, required ...string) llm.Tool {
	params := map[string]any{"type": "object", "properties": map[string]any(p)}
	if len(required) > 0 {
		params["required"] = required
	}
	return llm.Tool{Type: "function", Function: llm.ToolFuncDef{Name: name, Description: desc, Parameters: params}}
}

// Args is a loosely typed argument bag; the model sometimes sends numbers as
// strings or strings as numbers, so accessors coerce.
type Args map[string]any

// ParseArgs decodes tool-call arguments tolerantly.
func ParseArgs(raw string) (Args, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Args{}, nil
	}
	var a Args
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, err
	}
	return a, nil
}

func (a Args) Str(k string) (string, bool) {
	v, okk := a[k]
	if !okk || v == nil {
		return "", false
	}
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(t), true
	default:
		b, _ := json.Marshal(t)
		return string(b), true
	}
}

func (a Args) Int(k string, def int) int {
	v, okk := a[k]
	if !okk || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return def
}

// Exec dispatches the file/shell tools. update_plan, delegate and finish are
// handled by the loop.
func (e *Env) Exec(ctx context.Context, name string, a Args) Result {
	switch name {
	case "read_file":
		return e.readFile(a)
	case "write_file":
		return e.writeFile(a)
	case "edit_file":
		return e.editFile(a)
	case "run":
		return e.run(ctx, a)
	case "grep":
		return e.grep(a)
	case "list_dir":
		return e.listDir(a)
	}
	return fail("unknown_tool", "no tool named %q. Available: read_file, write_file, edit_file, run, grep, list_dir, update_plan, delegate, finish", name)
}

func (e *Env) readFile(a Args) Result {
	p, okk := a.Str("path")
	if !okk {
		return fail("missing_arg", "read_file needs path")
	}
	abs, err := e.Resolve(p)
	if err != nil {
		return fail("bad_path", "%v", err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return fail("not_found", "%s does not exist. Use list_dir or grep to locate files", p)
	}
	if st.IsDir() {
		return fail("is_dir", "%s is a directory; use list_dir", p)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fail("io", "%v", err)
	}
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return fail("binary", "%s looks binary (%d bytes)", p, len(data))
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	maxN := e.ReadMax
	if maxN <= 0 {
		maxN = 400
	}
	start := a.Int("start_line", 1)
	if start < 1 {
		start = 1
	}
	end := a.Int("end_line", start+maxN-1)
	if end > len(lines) {
		end = len(lines)
	}
	if end-start+1 > maxN {
		end = start + maxN - 1
	}
	if len(lines) == 0 {
		return ok("OK %s is empty", e.rel(abs))
	}
	if start > len(lines) {
		return fail("range", "%s has only %d lines", p, len(lines))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "OK %s lines %d-%d of %d\n", e.rel(abs), start, end, len(lines))
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%5d\t%s\n", i, lines[i-1])
	}
	if end < len(lines) {
		fmt.Fprintf(&b, "... (%d more lines; call read_file with start_line=%d)\n", len(lines)-end, end+1)
	}
	return ok("%s", b.String())
}

func (e *Env) writeFile(a Args) Result {
	p, okk := a.Str("path")
	if !okk {
		return fail("missing_arg", "write_file needs path")
	}
	content, okk := a.Str("content")
	if !okk {
		return fail("missing_arg", "write_file needs content")
	}
	abs, err := e.Resolve(p)
	if err != nil {
		return fail("bad_path", "%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fail("io", "%v", err)
	}
	_, existed := os.Stat(abs)
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return fail("io", "%v", err)
	}
	verb := "created"
	if existed == nil {
		verb = "overwrote"
	}
	n := strings.Count(content, "\n")
	if content != "" && !strings.HasSuffix(content, "\n") {
		n++
	}
	r := ok("OK %s %s (%d lines, %d bytes)", verb, e.rel(abs), n, len(content))
	r.Changed = []string{e.rel(abs)}
	return r
}

func (e *Env) editFile(a Args) Result {
	p, okk := a.Str("path")
	if !okk {
		return fail("missing_arg", "edit_file needs path")
	}
	oldT, ok1 := a.Str("old_text")
	newT, ok2 := a.Str("new_text")
	if !ok1 || !ok2 {
		return fail("missing_arg", "edit_file needs old_text and new_text")
	}
	if oldT == "" {
		return fail("missing_arg", "old_text is empty; to create a file use write_file")
	}
	abs, err := e.Resolve(p)
	if err != nil {
		return fail("bad_path", "%v", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return fail("not_found", "%s does not exist", p)
	}
	s := string(data)
	n := strings.Count(s, oldT)
	if n == 0 {
		// Common small-model slip: whitespace differences. Offer a hint.
		hint := "re-read the file with read_file and copy old_text exactly"
		if strings.Count(collapseWS(s), collapseWS(oldT)) > 0 {
			hint = "the text exists but whitespace/indentation differs (tabs vs spaces?); " + hint
		}
		return fail("no_match", "old_text not found in %s; %s", p, hint)
	}
	if n > 1 {
		return fail("ambiguous", "old_text matches %d times in %s; include more surrounding lines to make it unique", n, p)
	}
	s = strings.Replace(s, oldT, newT, 1)
	if err := os.WriteFile(abs, []byte(s), 0o644); err != nil {
		return fail("io", "%v", err)
	}
	idx := strings.Index(s, newT)
	line := strings.Count(s[:max(idx, 0)], "\n") + 1
	r := ok("OK edited %s at line %d (-%d +%d lines)", e.rel(abs), line, strings.Count(oldT, "\n")+1, strings.Count(newT, "\n")+1)
	r.Changed = []string{e.rel(abs)}
	return r
}

func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

var testCmdRe = regexp.MustCompile(`\b(go (test|build|vet)|npm (run )?(test|build)|pytest|make|cargo (test|build))\b`)

func (e *Env) run(ctx context.Context, a Args) Result {
	cmdline, okk := a.Str("command")
	if !okk || strings.TrimSpace(cmdline) == "" {
		return fail("missing_arg", "run needs command")
	}
	to := time.Duration(a.Int("timeout_s", 120)) * time.Second
	if to <= 0 {
		to = 120 * time.Second
	}
	maxRun := e.MaxRun
	if maxRun == 0 {
		maxRun = 600 * time.Second
	}
	if to > maxRun {
		to = maxRun
	}
	out, code, timedOut := Shell(ctx, e.Root, e.RunEnv, cmdline, to)
	var b strings.Builder
	status := "OK"
	if timedOut {
		status = fmt.Sprintf("ERROR timeout: command killed after %s", to)
	} else if code != 0 {
		status = fmt.Sprintf("ERROR exit_%d: command failed", code)
	}
	fmt.Fprintf(&b, "%s (exit=%d)\n", status, code)
	if strings.TrimSpace(out) == "" {
		b.WriteString("(no output)\n")
	} else {
		b.WriteString(out)
	}
	r := Result{OK: code == 0 && !timedOut, Output: b.String()}
	if timedOut {
		r.ErrorClass = "timeout"
	} else if code != 0 {
		r.ErrorClass = "exit_nonzero"
	}
	if testCmdRe.MatchString(cmdline) {
		if r.OK {
			r.TestSignal = "pass"
		} else {
			r.TestSignal = "fail"
		}
	}
	return r
}

// Shell runs cmdline with bash in dir, killing the whole process group on timeout.
func Shell(ctx context.Context, dir string, env []string, cmdline string, to time.Duration) (string, int, bool) {
	cctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	cmd := exec.Command("bash", "-c", cmdline)
	cmd.Dir = dir
	cmd.Env = env
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Stdin = nil
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err.Error(), 127, false
	}
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()
	var err error
	timedOut := false
	select {
	case err = <-waitDone:
	case <-cctx.Done():
		timedOut = true
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		err = <-waitDone
	}
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = 1
		}
		if timedOut {
			code = 124
		}
	}
	return buf.String(), code, timedOut
}

var skipDirs = map[string]bool{".git": true, ".qh": true, "node_modules": true, "vendor": true, ".cache": true}

func (e *Env) grep(a Args) Result {
	pat, okk := a.Str("pattern")
	if !okk || pat == "" {
		return fail("missing_arg", "grep needs pattern")
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		// Small models often send PCRE or unescaped literals; fall back to literal.
		re = regexp.MustCompile(regexp.QuoteMeta(pat))
	}
	p, _ := a.Str("path")
	root, err := e.Resolve(p)
	if err != nil {
		return fail("bad_path", "%v", err)
	}
	glob, _ := a.Str("glob")
	var hits []string
	files := 0
	const maxHits = 150
	truncated := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if glob != "" {
			if m, _ := filepath.Match(glob, d.Name()); !m {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil || info.Size() > 2<<20 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			return nil
		}
		files++
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				if len(hits) >= maxHits {
					truncated = true
					return filepath.SkipAll
				}
				if len(line) > 240 {
					line = line[:240] + "..."
				}
				hits = append(hits, fmt.Sprintf("%s:%d: %s", e.rel(path), i+1, line))
			}
		}
		return nil
	})
	if len(hits) == 0 {
		return ok("OK no matches for %q (%d files searched)", pat, files)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "OK %d matches\n", len(hits))
	b.WriteString(strings.Join(hits, "\n"))
	b.WriteString("\n")
	if truncated {
		b.WriteString("... (truncated; narrow the pattern or path)\n")
	}
	return ok("%s", b.String())
}

func (e *Env) listDir(a Args) Result {
	p, _ := a.Str("path")
	root, err := e.Resolve(p)
	if err != nil {
		return fail("bad_path", "%v", err)
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return fail("not_found", "%s is not a directory", p)
	}
	depth := a.Int("depth", 2)
	if depth < 1 {
		depth = 1
	}
	if depth > 5 {
		depth = 5
	}
	var b strings.Builder
	fmt.Fprintf(&b, "OK %s\n", e.rel(root))
	count := 0
	var walk func(dir string, lvl int)
	walk = func(dir string, lvl int) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
		for _, en := range ents {
			if count > 400 {
				return
			}
			count++
			ind := strings.Repeat("  ", lvl)
			if en.IsDir() {
				if skipDirs[en.Name()] {
					fmt.Fprintf(&b, "%s%s/ (skipped)\n", ind, en.Name())
					continue
				}
				fmt.Fprintf(&b, "%s%s/\n", ind, en.Name())
				if lvl+1 < depth {
					walk(filepath.Join(dir, en.Name()), lvl+1)
				}
			} else {
				info, _ := en.Info()
				sz := int64(0)
				if info != nil {
					sz = info.Size()
				}
				fmt.Fprintf(&b, "%s%s (%d B)\n", ind, en.Name(), sz)
			}
		}
	}
	walk(root, 0)
	if count > 400 {
		b.WriteString("... (truncated)\n")
	}
	return ok("%s", b.String())
}
