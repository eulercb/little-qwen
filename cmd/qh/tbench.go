package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/eulercb/little-qwen/internal/eval"
	"github.com/eulercb/little-qwen/internal/llm"
)

// cmdTBench runs Terminal-Bench 2 tasks (Tier C) with qh inside the task
// containers:
//
//	qh tbench --tasks <terminal-bench-2 checkout> --out eval/results/<name> [--only a,b] [--oracle] [--reps N]
//
// Needs a running docker daemon (eval/runner/docker-up.sh) and a static qh
// binary for the containers (built automatically into bin/qh-static).
func cmdTBench(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tbench", flag.ExitOnError)
	tasksDir := fs.String("tasks", "/home/user/laude-institute/terminal-bench-2", "terminal-bench-2 checkout")
	only := fs.String("only", "", "comma-separated task ids, or @file with one id per line")
	out := fs.String("out", "", "results dir (required)")
	reps := fs.Int("reps", 1, "repetitions")
	hdir := fs.String("harness", "harness/base", "harness dir")
	oracle := fs.Bool("oracle", false, "run the reference solution instead of the agent")
	mult := fs.Float64("timeout-mult", 4, "multiplier on each task's agent timeout (the local model is slow)")
	toolBin := fs.String("tool-bin", "bin/qh-static", "static qh binary for containers")
	uvDir := fs.String("uv-dir", "/tmp/qh-tbcache/uv", "static uv/uvx for the verifiers (eval/runner/tb-cache.sh)")
	fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("--out required")
	}
	var ids []string
	if strings.HasPrefix(*only, "@") {
		b, err := os.ReadFile((*only)[1:])
		if err != nil {
			return err
		}
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				ids = append(ids, l)
			}
		}
	} else if *only != "" {
		ids = strings.Split(*only, ",")
	}
	tasks, err := eval.LoadTBTasks(*tasksDir, ids)
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		return fmt.Errorf("no tasks found in %s", *tasksDir)
	}
	if _, err := os.Stat(*toolBin); err != nil {
		return fmt.Errorf("%s missing: build it with CGO_ENABLED=0 go build -o %s ./cmd/qh", *toolBin, *toolBin)
	}
	tb, _ := filepath.Abs(*toolBin)
	h, err := loadHarness(*hdir)
	if err != nil {
		return err
	}
	var c *llm.Client
	if !*oracle {
		if c, err = llm.New(); err != nil {
			return err
		}
		c.Log = logger("")
	}
	for rep := 1; rep <= *reps; rep++ {
		for _, t := range tasks {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			od := filepath.Join(*out, "tb-"+t.ID, fmt.Sprintf("r%d", rep))
			if doneOK(filepath.Join(od, "result.json")) {
				continue
			}
			lg := logger(fmt.Sprintf("%s/r%d ", t.ID, rep))
			lg("start image=%s timeout=%s×%.0f", t.Image, t.AgentTimeout, *mult)
			r, err := eval.RunTB(ctx, c, h, t, rep, od, eval.TBOptions{ToolBin: tb, Oracle: *oracle, TimeoutMult: *mult, UVDir: *uvDir}, lg)
			if err != nil {
				lg("ERROR %v", err)
				continue
			}
			lg("DONE reward=%.2f status=%s turns=%d wall=%.1fmin %s", r.Score, r.Outcome.Status, r.Outcome.Turns, r.Outcome.WallSeconds/60, r.Outcome.Err)
		}
	}
	res, err := eval.LoadResults(*out)
	if err != nil {
		return err
	}
	s := eval.Summary(res)
	_ = os.WriteFile(filepath.Join(*out, "summary.md"), []byte(s), 0o644)
	fmt.Print(s)
	return nil
}
