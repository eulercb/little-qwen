package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/eulercb/little-qwen/internal/eval"
	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/loop"
	"github.com/eulercb/little-qwen/internal/tools"
	"github.com/eulercb/little-qwen/internal/trace"
)

// cmdPropose runs the executor model as a Harness²-style editor:
//
//	qh propose --harness harness/base --editor harness/editor --out harness/cand/<name> [--focus "..."] <run-dir>...
//
// The editor works in a scratch dir holding evidence/ (digests + final diffs,
// never scores or hidden output) and harness/ (a copy to edit). The edited
// tree must pass `qh validate`; it is then copied to --out with PROPOSAL.md.
func cmdPropose(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("propose", flag.ExitOnError)
	hdir := fs.String("harness", "harness/base", "harness to improve")
	edir := fs.String("editor", "harness/editor", "editor harness")
	out := fs.String("out", "", "candidate output dir (required)")
	focus := fs.String("focus", "", "optional focus for the editor (e.g. an INV or DOM scope)")
	work := fs.String("work", "/tmp/qh-edit", "scratch root")
	fs.Parse(args)
	if *out == "" || fs.NArg() == 0 {
		return fmt.Errorf("usage: qh propose --out <dir> [flags] <run-dir>...")
	}
	editor, err := loadHarness(*edir)
	if err != nil {
		return err
	}
	wd := filepath.Join(*work, filepath.Base(*out))
	_ = os.RemoveAll(wd)
	if err := os.MkdirAll(filepath.Join(wd, "evidence"), 0o755); err != nil {
		return err
	}
	if err := copyDir(*hdir, filepath.Join(wd, "harness")); err != nil {
		return err
	}
	for i, rd := range fs.Args() {
		d, err := eval.DigestWith(rd, 600, 400)
		if err != nil {
			return err
		}
		name := fmt.Sprintf("run%02d-%s", i+1, strings.ReplaceAll(strings.TrimPrefix(rd, "eval/results/"), "/", "_"))
		if b, err := os.ReadFile(filepath.Join(rd, "final.diff")); err == nil {
			d += "\n\n# Final diff of the executor's work\n" + string(b)
		}
		if err := os.WriteFile(filepath.Join(wd, "evidence", name+".md"), []byte(d), 0o644); err != nil {
			return err
		}
	}
	self, _ := os.Executable()
	check := fmt.Sprintf("%s validate harness && test -s PROPOSAL.md", self)
	prompt := "Improve the execution harness in `harness/` using the run evidence in `evidence/`, following your instructions. " +
		"Read the evidence first (start with the digest headers), pick the single most costly failure or waste you can trace to a turn, and make one targeted edit. Write PROPOSAL.md."
	if *focus != "" {
		prompt += "\n\nFocus: " + *focus
	}
	c, err := llm.New()
	if err != nil {
		return err
	}
	c.Log = logger("")
	tr, err := trace.Open(filepath.Join(wd, ".qh", "trace"))
	if err != nil {
		return err
	}
	defer tr.Close()
	_ = exec("git init -q && git add -A && git -c user.name=qh -c user.email=qh@local commit -qm init", wd)
	ag := loop.New(c, editor, &tools.Env{Root: wd, RunEnv: eval.RunEnv(wd)}, tr, loop.Task{ID: "propose", Prompt: prompt, Check: check})
	ag.Log = logger("[editor] ")
	o := ag.Run(ctx)
	fmt.Printf("editor status=%s check_passed=%v turns=%d\n", o.Status, o.CheckPassed, o.Turns)
	if !o.CheckPassed {
		return fmt.Errorf("editor did not produce a valid harness (workdir %s)", wd)
	}
	_ = os.RemoveAll(*out)
	if err := copyDir(filepath.Join(wd, "harness"), *out); err != nil {
		return err
	}
	b, _ := os.ReadFile(filepath.Join(wd, "PROPOSAL.md"))
	return os.WriteFile(filepath.Join(*out, "PROPOSAL.md"), b, 0o644)
}

func exec(cmd, dir string) error {
	out, code, _ := tools.Shell(context.Background(), dir, nil, cmd, 60e9)
	if code != 0 {
		return fmt.Errorf("%s: %s", cmd, out)
	}
	return nil
}

func copyDir(src, dst string) error {
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
