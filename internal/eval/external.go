package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eulercb/little-qwen/internal/loop"
	"github.com/eulercb/little-qwen/internal/tools"
)

// RunExternal runs a task with an external agent command (e.g. stock
// OpenCode) instead of qh, for baseline comparisons. The command runs in
// the workdir with QH_PROMPT_FILE, QH_WORKDIR and QH_TIMEOUT_S set; its
// output goes to <outDir>/agent.log.
func RunExternal(ctx context.Context, agentCmd string, t TaskSpec, rep int, outDir, workRoot string, timeout time.Duration, logf func(string, ...any)) (*Result, error) {
	work := filepath.Join(workRoot, fmt.Sprintf("%s-r%d", t.ID, rep))
	if err := PrepareWorkdir(ctx, t, work); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(t.Prompt) + "\n"
	if t.Check != "" {
		prompt += fmt.Sprintf("\nAcceptance check: `%s` must exit 0. Run it yourself before you finish.\n", t.Check)
	}
	pf := filepath.Join(outDir, "prompt.txt")
	if err := os.WriteFile(pf, []byte(prompt), 0o644); err != nil {
		return nil, err
	}
	env := append(RunEnv(work), "QH_PROMPT_FILE="+pf, "QH_WORKDIR="+work, fmt.Sprintf("QH_TIMEOUT_S=%d", int(timeout.Seconds())))
	for _, k := range []string{"REMOTE_LLM_ENDPOINT", "QH_ENDPOINT", "QH_MODEL"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	start := time.Now()
	logf("external agent start")
	out, code, timedOut := tools.Shell(ctx, work, env, agentCmd, timeout+2*time.Minute)
	_ = os.WriteFile(filepath.Join(outDir, "agent.log"), []byte(out), 0o644)
	o := loop.Outcome{Status: "finished", WallSeconds: time.Since(start).Seconds()}
	if timedOut {
		o.Status = "wall_clock"
	} else if code != 0 {
		o.Status = fmt.Sprintf("exit_%d", code)
	}
	if t.Check != "" {
		_, c, _ := tools.Shell(ctx, work, RunEnv(work), t.Check, 10*time.Minute)
		o.CheckPassed = c == 0
	}
	res := &Result{TaskID: t.ID, Tier: t.Tier, Rep: rep, HarnessHash: "external", Outcome: o, Started: start.UTC().Format(time.RFC3339)}
	saveDiff(ctx, work, outDir)
	scoreHidden(ctx, t, work, res)
	b, _ := json.MarshalIndent(res, "", "  ")
	return res, os.WriteFile(filepath.Join(outDir, "result.json"), b, 0o644)
}
