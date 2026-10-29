package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	pf, err := filepath.Abs(filepath.Join(outDir, "prompt.txt"))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(pf, []byte(prompt), 0o644); err != nil {
		return nil, err
	}
	// The raw task prompt (without the generic check line) and the check
	// itself, for agents that build their own task card (pi + pi-qh).
	rawPf := filepath.Join(filepath.Dir(pf), "prompt_raw.txt")
	if err := os.WriteFile(rawPf, []byte(strings.TrimSpace(t.Prompt)+"\n"), 0o644); err != nil {
		return nil, err
	}
	absOut, _ := filepath.Abs(outDir)
	env := append(RunEnv(work), "QH_PROMPT_FILE="+pf, "QH_RAW_PROMPT_FILE="+rawPf, "QH_CHECK="+t.Check, "QH_AGENT_OUT="+absOut,
		"QH_WORKDIR="+work, fmt.Sprintf("QH_TIMEOUT_S=%d", int(timeout.Seconds())))
	for _, k := range []string{"REMOTE_LLM_ENDPOINT", "QH_ENDPOINT", "QH_MODEL", "QH_PI_CONFIG", "QH_PI_OFF", "QH_PI_CHUNKS"} {
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
	// Agents that report their own outcome (pi-qh writes agent_status.json).
	if b, err := os.ReadFile(filepath.Join(outDir, "agent_status.json")); err == nil && !timedOut {
		var st struct {
			Status string `json:"status"`
			Turns  int    `json:"turns"`
		}
		if json.Unmarshal(b, &st) == nil && st.Status != "" {
			o.Status, o.Turns = st.Status, st.Turns
		}
	}
	if endpointDown(ctx, out) {
		// The agent died on a tunnel outage (it cannot tell us itself): mark the
		// run as an endpoint error so qh eval stops and a resume re-runs it.
		o.Status, o.Err = "error", "endpoint unavailable (external agent hit an outage)"
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

// endpointDown reports whether an external run was hit by an endpoint outage:
// a Cloudflare error page in the agent's output, or a failing /health now.
func endpointDown(ctx context.Context, agentOut string) bool {
	if strings.Contains(agentOut, "cf-error-details") || strings.Contains(agentOut, "trycloudflare.com | 5") {
		return true
	}
	ep := os.Getenv("QH_ENDPOINT")
	if ep == "" {
		ep = os.Getenv("REMOTE_LLM_ENDPOINT")
	}
	if ep == "" {
		return false
	}
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, "GET", strings.TrimRight(ep, "/")+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return true
	}
	resp.Body.Close()
	return resp.StatusCode != 200
}
