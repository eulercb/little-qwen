// Command qh is a small agent harness for Qwen3.8-27B served behind an
// OpenAI-compatible endpoint.
//
//	qh run      --dir <workdir> --prompt <text|@file> [--check <cmd>] [--harness harness/base]
//	qh eval     --tasks eval/tasks --harness harness/base --out eval/results/<iter> [--reps 1] [--only a,b] [--parallel 1]
//	qh report   <results-dir>
//	qh digest   <run-dir>
//	qh validate <harness-dir>
//	qh smoke    [--n 50]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/eulercb/little-qwen/internal/eval"
	"github.com/eulercb/little-qwen/internal/harness"
	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/loop"
	"github.com/eulercb/little-qwen/internal/tools"
	"github.com/eulercb/little-qwen/internal/trace"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var err error
	switch os.Args[1] {
	case "run":
		err = cmdRun(ctx, os.Args[2:])
	case "eval":
		err = cmdEval(ctx, os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "digest":
		err = cmdDigest(os.Args[2:])
	case "validate":
		err = cmdValidate(os.Args[2:])
	case "smoke":
		err = cmdSmoke(ctx, os.Args[2:])
	case "propose":
		err = cmdPropose(ctx, os.Args[2:])
	case "rescore":
		err = cmdRescore(ctx, os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "qh:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: qh run|eval|report|digest|validate|smoke [flags]")
	os.Exit(2)
}

func logger(prefix string) func(string, ...any) {
	var mu sync.Mutex
	return func(format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		fmt.Fprintf(os.Stderr, "%s %s"+format+"\n", append([]any{time.Now().Format("15:04:05"), prefix}, args...)...)
	}
}

func loadHarness(dir string) (*harness.Harness, error) {
	h, err := harness.Load(dir)
	if err != nil {
		return nil, err
	}
	if probs := h.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("harness %s invalid:\n  %s", dir, strings.Join(probs, "\n  "))
	}
	return h, nil
}

func cmdRun(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	dir := fs.String("dir", ".", "workdir")
	prompt := fs.String("prompt", "", "task text or @file")
	check := fs.String("check", "", "acceptance command")
	hdir := fs.String("harness", "harness/base", "harness dir")
	traceDir := fs.String("trace", "", "trace dir (default <dir>/.qh/trace)")
	fs.Parse(args)
	p := *prompt
	if strings.HasPrefix(p, "@") {
		b, err := os.ReadFile(p[1:])
		if err != nil {
			return err
		}
		p = string(b)
	}
	if p == "" {
		return fmt.Errorf("--prompt required")
	}
	h, err := loadHarness(*hdir)
	if err != nil {
		return err
	}
	abs, _ := filepath.Abs(*dir)
	if err := h.Materialize(abs); err != nil {
		return err
	}
	c, err := llm.New()
	if err != nil {
		return err
	}
	c.Log = logger("")
	td := *traceDir
	if td == "" {
		td = filepath.Join(abs, ".qh", "trace")
	}
	tr, err := trace.Open(td)
	if err != nil {
		return err
	}
	defer tr.Close()
	ag := loop.New(c, h, &tools.Env{Root: abs, RunEnv: eval.RunEnv(abs)}, tr, loop.Task{ID: "adhoc", Prompt: p, Check: *check})
	ag.Log = logger("")
	o := ag.Run(ctx)
	fmt.Printf("status=%s check_passed=%v turns=%d compactions=%d wall=%.1fmin\nsummary: %s\n", o.Status, o.CheckPassed, o.Turns, o.Compactions, o.WallSeconds/60, o.Summary)
	return nil
}

func cmdEval(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	tasksDir := fs.String("tasks", "eval/tasks", "tasks dir")
	hdir := fs.String("harness", "harness/base", "harness dir")
	out := fs.String("out", "", "results dir (required)")
	reps := fs.Int("reps", 1, "repetitions per task")
	only := fs.String("only", "", "comma-separated task ids or tiers")
	par := fs.Int("parallel", 1, "concurrent runs")
	work := fs.String("work", "", "work root (default $QH_RUNS or /tmp/qh-runs)")
	skipDone := fs.Bool("resume", true, "skip runs that already have result.json")
	agent := fs.String("agent", "", "external agent command (e.g. eval/runner/opencode_agent.sh); default: qh itself")
	extTimeout := fs.Int("agent-timeout-min", 60, "timeout for external agents")
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("--out required")
	}
	if *agent != "" {
		// The agent command runs inside the task workdir; anchor a relative
		// script path to the current directory.
		if f := strings.Fields(*agent); len(f) > 0 && !filepath.IsAbs(f[0]) && strings.Contains(f[0], "/") {
			if abs, err := filepath.Abs(f[0]); err == nil {
				*agent = abs + strings.TrimPrefix(*agent, f[0])
			}
		}
	}
	h, err := loadHarness(*hdir)
	if err != nil {
		return err
	}
	// Advertise the running eval so queues never start a concurrent one.
	if lock := filepath.Join("eval", ".eval.lock"); dirExists("eval") {
		_ = os.WriteFile(lock, []byte(fmt.Sprint(os.Getpid())), 0o644)
		defer os.Remove(lock)
	}
	var filter []string
	if *only != "" {
		filter = strings.Split(*only, ",")
	}
	tasks, err := eval.LoadTasks(*tasksDir, filter)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks")
	}
	c, err := llm.New()
	if err != nil {
		return err
	}
	c.Log = logger("")
	wr := *work
	if wr == "" {
		wr = os.Getenv("QH_RUNS")
	}
	if wr == "" {
		wr = "/tmp/qh-runs"
	}
	wr = filepath.Join(wr, filepath.Base(*out))
	type job struct {
		t   eval.TaskSpec
		rep int
	}
	var jobs []job
	for rep := 1; rep <= *reps; rep++ {
		for _, t := range tasks {
			jobs = append(jobs, job{t, rep})
		}
	}
	ch := make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < *par; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				od := filepath.Join(*out, j.t.ID, fmt.Sprintf("r%d", j.rep))
				if *skipDone && doneOK(filepath.Join(od, "result.json")) {
					continue
				}
				_ = os.RemoveAll(od)
				// Write into a staging dir and move it into place when the run
				// ends, so the results tree only ever holds finished runs.
				final := od
				od = filepath.Join(*out, ".inprogress", j.t.ID, fmt.Sprintf("r%d", j.rep))
				_ = os.RemoveAll(od)
				lg := logger(fmt.Sprintf("%s/r%d ", j.t.ID, j.rep))
				lg("start (harness %s, agent %q)", h.Hash, *agent)
				var r *eval.Result
				var err error
				if *agent != "" {
					r, err = eval.RunExternal(ctx, *agent, j.t, j.rep, od, wr, time.Duration(*extTimeout)*time.Minute, lg)
				} else {
					r, err = eval.RunOne(ctx, c, h, j.t, j.rep, od, wr, lg)
				}
				if err != nil {
					lg("ERROR %v", err)
					continue
				}
				_ = os.MkdirAll(filepath.Dir(final), 0o755)
				if err := os.Rename(od, final); err != nil {
					lg("ERROR moving results: %v", err)
				}
				lg("DONE success=%v score=%.2f status=%s turns=%d wall=%.1fmin", r.Success, r.Score, r.Outcome.Status, r.Outcome.Turns, r.Outcome.WallSeconds/60)
			}
		}()
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		ch <- j
	}
	close(ch)
	wg.Wait()
	_ = os.RemoveAll(filepath.Join(*out, ".inprogress"))
	res, err := eval.LoadResults(*out)
	if err != nil {
		return err
	}
	s := eval.Summary(res)
	_ = os.WriteFile(filepath.Join(*out, "summary.md"), []byte(s), 0o644)
	fmt.Print(s)
	return nil
}

func cmdReport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: qh report <results-dir>")
	}
	res, err := eval.LoadResults(args[0])
	if err != nil {
		return err
	}
	fmt.Print(eval.Summary(res))
	return nil
}

func cmdDigest(args []string) error {
	fs := flag.NewFlagSet("digest", flag.ExitOnError)
	max := fs.Int("max", 400, "max lines")
	thoughts := fs.Int("thoughts", 0, "include up to N chars of each turn's reasoning")
	fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: qh digest <run-dir>")
	}
	d, err := eval.DigestWith(fs.Arg(0), *max, *thoughts)
	if err != nil {
		return err
	}
	fmt.Print(d)
	return nil
}

func cmdValidate(args []string) error {
	dir := "harness/base"
	if len(args) > 0 {
		dir = args[0]
	}
	h, err := harness.Load(dir)
	if err != nil {
		return err
	}
	fmt.Printf("harness %s hash=%s always-on=%d chars (~%d tokens) skills=%d scripts=%d sub_agents=%d\n",
		dir, h.Hash, len(h.System), len(h.System)*10/33, len(h.Skills), len(h.Scripts), len(h.SubAgents))
	if probs := h.Validate(); len(probs) > 0 {
		return fmt.Errorf("invalid:\n  %s", strings.Join(probs, "\n  "))
	}
	fmt.Println("OK")
	return nil
}

// cmdRescore: qh rescore [--tasks eval/tasks] [--work-root /tmp/qh-runs] <results-dir>
func cmdRescore(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("rescore", flag.ExitOnError)
	tasksDir := fs.String("tasks", "eval/tasks", "tasks dir")
	workRoot := fs.String("work-root", "/tmp/qh-runs", "fallback work root for results without a recorded workdir")
	fs.Parse(args)
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: qh rescore <results-dir>")
	}
	dir := fs.Arg(0)
	tasks, err := eval.LoadTasks(*tasksDir, nil)
	if err != nil {
		return err
	}
	byID := map[string]eval.TaskSpec{}
	for _, t := range tasks {
		byID[t.ID] = t
	}
	results, err := eval.LoadResults(dir)
	if err != nil {
		return err
	}
	for _, r := range results {
		t, ok := byID[r.TaskID]
		if !ok {
			continue
		}
		rp := filepath.Join(dir, r.TaskID, fmt.Sprintf("r%d", r.Rep), "result.json")
		if r.Work == "" {
			// Backfill the workdir path for runs made before it was recorded.
			r.Work = filepath.Join(*workRoot, filepath.Base(dir), fmt.Sprintf("%s-r%d", r.TaskID, r.Rep))
			b, _ := json.MarshalIndent(&r, "", "  ")
			_ = os.WriteFile(rp, b, 0o644)
		}
		nr, err := eval.Rescore(ctx, t, rp)
		if err != nil {
			fmt.Printf("%s r%d: %v\n", r.TaskID, r.Rep, err)
			continue
		}
		fmt.Printf("%s r%d: success %v -> %v score %.2f -> %.2f\n", r.TaskID, r.Rep, r.Success, nr.Success, r.Score, nr.Score)
	}
	res, _ := eval.LoadResults(dir)
	s := eval.Summary(res)
	return os.WriteFile(filepath.Join(dir, "summary.md"), []byte(s), 0o644)
}

// doneOK reports whether a run finished without an infrastructure error, so
// resumed evals re-run runs that died because the endpoint was unreachable.
func doneOK(resultPath string) bool {
	b, err := os.ReadFile(resultPath)
	if err != nil {
		return false
	}
	var r eval.Result
	if json.Unmarshal(b, &r) != nil {
		return false
	}
	return r.Outcome.Status != "error"
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
