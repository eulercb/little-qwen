// Package eval runs tasks against a harness and scores them with hidden
// checks the executor (and the harness editor) never sees.
package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eulercb/little-qwen/internal/harness"
	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/loop"
	"github.com/eulercb/little-qwen/internal/tools"
	"github.com/eulercb/little-qwen/internal/trace"
)

// TaskSpec is eval/tasks/<id>/task.json. The prompt lives in prompt.md,
// starting files in repo/, and hidden scoring files in hidden/.
type TaskSpec struct {
	ID          string `json:"id"`
	Tier        string `json:"tier"`
	Domain      string `json:"domain"`
	Check       string `json:"check"`        // visible acceptance command
	HiddenCheck string `json:"hidden_check"` // run after copying hidden/ over the workdir
	Score       string `json:"score"`        // "gotest" (fraction of passing tests) or "exit"
	Setup       string `json:"setup"`        // optional command run in the workdir before the agent starts
	// Sessions, when set, runs one fresh agent session per entry, in order and
	// in the same workdir; each session's prompt is the task prompt followed by
	// the entry (e.g. "Do milestone M3 now."). Models multi-session work.
	Sessions []string `json:"sessions"`
	// Base, when set, names another task whose repo/, checker/, hidden/ and
	// prompt.md this task reuses; PromptSuffix is appended to the prompt
	// (e.g. d01s: the d01 brief as one long session).
	Base         string `json:"base"`
	PromptSuffix string `json:"prompt_suffix"`
	Dir          string `json:"-"`
	Prompt       string `json:"-"`
}

func LoadTasks(root string, filter []string) ([]TaskSpec, error) {
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, f := range filter {
		want[f] = true
	}
	var out []TaskSpec
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		b, err := os.ReadFile(filepath.Join(dir, "task.json"))
		if err != nil {
			continue
		}
		var t TaskSpec
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		if t.ID == "" {
			t.ID = e.Name()
		}
		if t.Base != "" {
			dir = filepath.Join(root, t.Base)
		}
		p, err := os.ReadFile(filepath.Join(dir, "prompt.md"))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		t.Prompt = string(p)
		if t.PromptSuffix != "" {
			t.Prompt = strings.TrimRight(t.Prompt, "\n") + "\n\n---\n\n" + t.PromptSuffix + "\n"
		}
		// Absolute: hidden checks run inside the workdir and use $QH_TASK_DIR
		// (a relative path scored d01 0 with "check.sh: No such file").
		if t.Dir, err = filepath.Abs(dir); err != nil {
			return nil, err
		}
		if len(want) > 0 && !want[t.ID] && !want[t.Tier] {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Result is written to <out>/result.json.
type Result struct {
	TaskID      string       `json:"task_id"`
	Tier        string       `json:"tier"`
	Rep         int          `json:"rep"`
	HarnessHash string       `json:"harness_hash"`
	Success     bool         `json:"success"`
	Score       float64      `json:"score"`
	TestsPassed int          `json:"tests_passed"`
	TestsTotal  int          `json:"tests_total"`
	Outcome     loop.Outcome `json:"outcome"`
	Started     string       `json:"started"`
	HiddenOut   string       `json:"hidden_output"`
	Work        string       `json:"work,omitempty"`
	Rescored    bool         `json:"rescored,omitempty"`
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		t := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(t, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		return os.WriteFile(t, b, info.Mode().Perm())
	})
}

// RunEnv is the environment for executor commands: offline Go, no endpoint
// credentials.
func RunEnv(work string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k := kv[:strings.IndexByte(kv, '=')]
		switch k {
		case "REMOTE_LLM_ENDPOINT", "QH_ENDPOINT", "ANTHROPIC_API_KEY", "GH_TOKEN", "GITHUB_TOKEN":
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GOFLAGS=-mod=mod", "GOTOOLCHAIN=local", "GOPROXY=off", "GIT_AUTHOR_NAME=qh", "GIT_AUTHOR_EMAIL=qh@local",
		"GIT_COMMITTER_NAME=qh", "GIT_COMMITTER_EMAIL=qh@local", "TERM=dumb", "NO_COLOR=1")
	return env
}

// PrepareWorkdir copies repo/ into work and commits it.
func PrepareWorkdir(ctx context.Context, t TaskSpec, work string) error {
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(t.Dir, "repo")); err == nil {
		if err := copyTree(filepath.Join(t.Dir, "repo"), work); err != nil {
			return err
		}
	}
	env := RunEnv(work)
	setup := "git init -q && mkdir -p .git/info && echo '.qh/' >> .git/info/exclude && git add -A && git commit -q -m init --allow-empty"
	if out, code, _ := tools.Shell(ctx, work, env, setup, time.Minute); code != 0 {
		return fmt.Errorf("git init: %s", out)
	}
	if t.Setup != "" {
		if out, code, _ := tools.Shell(ctx, work, env, t.Setup+" && git add -A && git commit -q -m setup --allow-empty", 10*time.Minute); code != 0 {
			return fmt.Errorf("setup: %s", out)
		}
	}
	return nil
}

// RunOne runs a single task repetition.
func RunOne(ctx context.Context, c *llm.Client, h *harness.Harness, t TaskSpec, rep int, outDir, workRoot string, logf func(string, ...any)) (*Result, error) {
	work := filepath.Join(workRoot, fmt.Sprintf("%s-r%d", t.ID, rep))
	if err := PrepareWorkdir(ctx, t, work); err != nil {
		return nil, err
	}
	if err := h.Materialize(work); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	started := time.Now().UTC().Format(time.RFC3339)
	var o loop.Outcome
	if len(t.Sessions) == 0 {
		tr, err := trace.Open(outDir)
		if err != nil {
			return nil, err
		}
		env := &tools.Env{Root: work, RunEnv: RunEnv(work)}
		ag := loop.New(c, h, env, tr, loop.Task{ID: t.ID, Prompt: t.Prompt, Check: t.Check})
		ag.Log = logf
		o = ag.Run(ctx)
		tr.Close()
	} else {
		o = runSessions(ctx, c, h, t, work, outDir, logf)
	}
	res := &Result{TaskID: t.ID, Tier: t.Tier, Rep: rep, HarnessHash: h.Hash, Outcome: o, Started: started, Work: work}
	// Keep the final diff as an artifact (before hidden files are copied in).
	saveDiff(ctx, work, outDir)
	scoreHidden(ctx, t, work, res)
	b, _ := json.MarshalIndent(res, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "result.json"), b, 0o644); err != nil {
		return nil, err
	}
	return res, nil
}

// runSessions runs t.Sessions one after another with fresh contexts in the
// same workdir. Traces go to <outDir>/s<k>/; the outcome sums the sessions.
func runSessions(ctx context.Context, c *llm.Client, h *harness.Harness, t TaskSpec, work, outDir string, logf func(string, ...any)) loop.Outcome {
	var total loop.Outcome
	total.Codes = map[string]int{}
	for k, sp := range t.Sessions {
		sd := filepath.Join(outDir, fmt.Sprintf("s%d", k))
		tr, err := trace.Open(sd)
		if err != nil {
			total.Status, total.Err = "error", err.Error()
			return total
		}
		env := &tools.Env{Root: work, RunEnv: RunEnv(work)}
		prompt := strings.TrimSpace(t.Prompt) + "\n\n---\n\n# This session\n" + sp
		ag := loop.New(c, h, env, tr, loop.Task{ID: fmt.Sprintf("%s#%d", t.ID, k), Prompt: prompt, Check: t.Check})
		ag.Log = func(f string, a ...any) { logf("[s%d] "+f, append([]any{k}, a...)...) }
		o := ag.Run(ctx)
		tr.Close()
		b, _ := json.MarshalIndent(o, "", "  ")
		_ = os.WriteFile(filepath.Join(sd, "outcome.json"), b, 0o644)
		logf("session %d (%s): status=%s check=%v turns=%d wall=%.1fmin", k, sp, o.Status, o.CheckPassed, o.Turns, o.WallSeconds/60)
		total.Turns += o.Turns
		total.Compactions += o.Compactions
		total.PromptTokens += o.PromptTokens
		total.PrefillTokens += o.PrefillTokens
		total.ReprefillTokens += o.ReprefillTokens
		total.CompletionTokens += o.CompletionTokens
		total.ReasoningTokens += o.ReasoningTokens
		total.ToolCalls += o.ToolCalls
		total.ToolErrors += o.ToolErrors
		total.Repaired += o.Repaired
		total.Nudges += o.Nudges
		total.LoopHints += o.LoopHints
		total.FinishAttempts += o.FinishAttempts
		total.Delegations += o.Delegations
		total.WallSeconds += o.WallSeconds
		for c, n := range o.Codes {
			total.Codes[c] += n
		}
		total.Status, total.CheckPassed, total.Summary = o.Status, o.CheckPassed, o.Summary
		if o.Status == "error" {
			total.Err = o.Err
			return total // endpoint/infra problem: stop here
		}
	}
	return total
}

func scoreHidden(ctx context.Context, t TaskSpec, work string, res *Result) {
	hidden := filepath.Join(t.Dir, "hidden")
	if _, err := os.Stat(hidden); err == nil {
		if err := copyTree(hidden, work); err != nil {
			res.HiddenOut = "copy hidden: " + err.Error()
			return
		}
	}
	cmd := t.HiddenCheck
	if cmd == "" {
		cmd = t.Check
	}
	if t.Score == "gotest" {
		cmd = strings.Replace(cmd, "go test", "go test -json", 1)
	}
	env := append(RunEnv(work), "QH_TASK_DIR="+t.Dir)
	out, code, _ := tools.Shell(ctx, work, env, cmd, 30*time.Minute)
	if t.Score == "fraction" {
		// The checker prints "SCORE <passed> <total>" as its last such line.
		for _, l := range strings.Split(out, "\n") {
			var p, n int
			if _, err := fmt.Sscanf(strings.TrimSpace(l), "SCORE %d %d", &p, &n); err == nil && n > 0 {
				res.TestsPassed, res.TestsTotal = p, n
			}
		}
		if res.TestsTotal > 0 {
			res.Score = float64(res.TestsPassed) / float64(res.TestsTotal)
		}
		res.Success = res.TestsTotal > 0 && res.TestsPassed == res.TestsTotal
		res.HiddenOut = tailLines(out, 80)
		return
	}
	if t.Score == "gotest" {
		pass, total := countGoTests(out)
		res.TestsPassed, res.TestsTotal = pass, total
		if total > 0 {
			res.Score = float64(pass) / float64(total)
		}
		res.Success = code == 0 && total > 0 && pass == total
		res.HiddenOut = tailLines(goTestText(out), 60)
	} else {
		res.Success = code == 0
		if res.Success {
			res.Score = 1
		}
		res.HiddenOut = tailLines(out, 60)
	}
}

type goTestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

// countGoTests counts top-level test results; a package that fails to build
// counts as one failed test.
func countGoTests(out string) (int, int) {
	pass, total := 0, 0
	pkgHadTest := map[string]bool{}
	pkgFail := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var e goTestEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		if e.Test != "" && !strings.Contains(e.Test, "/") {
			switch e.Action {
			case "pass":
				pass++
				total++
				pkgHadTest[e.Package] = true
			case "fail":
				total++
				pkgHadTest[e.Package] = true
			}
		}
		if e.Test == "" && e.Action == "fail" {
			pkgFail[e.Package] = true
		}
	}
	for p := range pkgFail {
		if !pkgHadTest[p] {
			total++
		}
	}
	if total == 0 && strings.Contains(out, "[build failed]") {
		total = 1
	}
	return pass, total
}

func goTestText(out string) string {
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var e goTestEvent
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			b.WriteString(sc.Text() + "\n")
			continue
		}
		if e.Action == "output" {
			b.WriteString(e.Output)
		}
	}
	return b.String()
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func saveDiff(ctx context.Context, work, outDir string) {
	diff, _, _ := tools.Shell(ctx, work, RunEnv(work), "git add -A -- . ':!.qh' ':!opencode.json' >/dev/null 2>&1; git diff --cached --stat; echo; git diff --cached | head -c 60000", time.Minute)
	_ = os.WriteFile(filepath.Join(outDir, "final.diff"), []byte(trace.Redact(diff)), 0o644)
}

// Rescore re-runs hidden scoring for an existing result whose workdir still
// exists (e.g. after fixing a hidden test). Hidden files are removed first so
// the saved diff only shows the agent's work.
func Rescore(ctx context.Context, t TaskSpec, resPath string) (*Result, error) {
	b, err := os.ReadFile(resPath)
	if err != nil {
		return nil, err
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.Work == "" {
		return nil, fmt.Errorf("%s: no workdir recorded", resPath)
	}
	if _, err := os.Stat(r.Work); err != nil {
		return nil, fmt.Errorf("workdir gone: %w", err)
	}
	hidden := filepath.Join(t.Dir, "hidden")
	_ = filepath.WalkDir(hidden, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(hidden, p)
			_ = os.Remove(filepath.Join(r.Work, rel))
		}
		return nil
	})
	// Older runs copied hidden tests under their old names too.
	_, _, _ = tools.Shell(ctx, r.Work, RunEnv(r.Work), "find . -name 'zz_hidden_test.go' -not -path './.git/*' -delete", time.Minute)
	saveDiff(ctx, r.Work, filepath.Dir(resPath))
	r.Success, r.Score, r.TestsPassed, r.TestsTotal, r.HiddenOut = false, 0, 0, 0, ""
	scoreHidden(ctx, t, r.Work, &r)
	r.Rescored = true
	nb, _ := json.MarshalIndent(&r, "", "  ")
	return &r, os.WriteFile(resPath, nb, 0o644)
}
