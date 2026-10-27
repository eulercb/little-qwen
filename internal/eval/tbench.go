package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eulercb/little-qwen/internal/harness"
	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/loop"
	"github.com/eulercb/little-qwen/internal/tools"
	"github.com/eulercb/little-qwen/internal/trace"
)

// TBTask is one Terminal-Bench 2 task directory (task.toml, instruction.md,
// environment/, tests/, solution/).
type TBTask struct {
	ID              string
	Dir             string
	Instruction     string
	Image           string
	AgentTimeout    time.Duration
	VerifierTimeout time.Duration
	AllowInternet   bool
	CPUs            string
	MemoryMB        string
	Difficulty      string
	Category        string
}

// parseTOML reads the flat `key = value` pairs of task.toml, keyed as
// "section.key". Enough for the fields the runner needs.
func parseTOML(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "[") {
			section = strings.Trim(l, "[]")
			continue
		}
		k, v, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		out[section+"."+strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"`)
	}
	return out, sc.Err()
}

// LoadTBTasks loads tasks from a terminal-bench-2 checkout.
func LoadTBTasks(root string, only []string) ([]TBTask, error) {
	want := map[string]bool{}
	for _, o := range only {
		want[o] = true
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []TBTask
	for _, e := range ents {
		dir := filepath.Join(root, e.Name())
		kv, err := parseTOML(filepath.Join(dir, "task.toml"))
		if err != nil || (len(want) > 0 && !want[e.Name()]) {
			continue
		}
		ins, err := os.ReadFile(filepath.Join(dir, "instruction.md"))
		if err != nil {
			continue
		}
		secs := func(k string, def float64) time.Duration {
			if f, err := strconv.ParseFloat(kv[k], 64); err == nil {
				return time.Duration(f * float64(time.Second))
			}
			return time.Duration(def * float64(time.Second))
		}
		out = append(out, TBTask{
			ID: e.Name(), Dir: dir, Instruction: string(ins), Image: kv["environment.docker_image"],
			AgentTimeout: secs("agent.timeout_sec", 900), VerifierTimeout: secs("verifier.timeout_sec", 900),
			AllowInternet: kv["environment.allow_internet"] != "false", CPUs: kv["environment.cpus"],
			MemoryMB: kv["environment.memory_mb"], Difficulty: kv["metadata.difficulty"], Category: kv["metadata.category"],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func docker(ctx context.Context, to time.Duration, args ...string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	out, err := exec.CommandContext(cctx, "docker", args...).CombinedOutput()
	return string(out), err
}

// containerNetArgs gives a container the session's egress: host network (the
// proxy listens on localhost), proxy env vars and the proxy CA as the system
// bundle (see eval/runner/docker-up.sh and /root/.ccr/README.md).
func containerNetArgs() []string {
	args := []string{"--network", "host"}
	for _, k := range []string{"HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "https_proxy", "http_proxy", "no_proxy"} {
		if v := os.Getenv(k); v != "" {
			args = append(args, "-e", k+"="+v)
		}
	}
	ca := "/root/.ccr/ca-bundle.crt"
	if _, err := os.Stat(ca); err == nil {
		const in = "/etc/ssl/certs/ca-certificates.crt"
		args = append(args, "-v", ca+":"+in+":ro", "-e", "SSL_CERT_FILE="+in, "-e", "REQUESTS_CA_BUNDLE="+in,
			"-e", "CURL_CA_BUNDLE="+in, "-e", "NODE_EXTRA_CA_CERTS="+in, "-e", "PIP_CERT="+in, "-e", "UV_NATIVE_TLS=1")
	}
	return args
}

// netSetup makes apt use HTTPS through the proxy (the proxy has no plain-HTTP lane).
const netSetup = `for f in /etc/apt/sources.list /etc/apt/sources.list.d/*; do [ -f "$f" ] && sed -i -E 's#http://(deb|security|archive|ports)\.(debian|ubuntu)\.(org|com)#https://\1.\2.\3#g' "$f"; done 2>/dev/null
if [ -d /etc/apt ] && [ -n "$HTTPS_PROXY$https_proxy" ]; then p="${HTTPS_PROXY:-$https_proxy}"; printf 'Acquire::http::Proxy "%s";\nAcquire::https::Proxy "%s";\n' "$p" "$p" > /etc/apt/apt.conf.d/99qh-proxy; fi
true`

// TBOptions configures a Terminal-Bench run.
type TBOptions struct {
	ToolBin     string  // static qh binary copied into the container
	Oracle      bool    // run solution/solve.sh instead of the agent (validates the setup)
	TimeoutMult float64 // multiplier on the task's agent timeout (the local model is slow)
	UVDir       string  // host dir with static uv/uvx, provisioned for the verifier (see eval/runner/tb-cache.sh)
	Agent       string  // external agent command run on the host (e.g. eval/runner/lc_tb_agent.py); "" = qh
}

// provisionUV puts uv/uvx and the env file the official verifiers `source`
// into /root/.local/bin, standing in for the astral.sh installer, which this
// environment's egress policy blocks. The tests themselves are unchanged.
func provisionUV(ctx context.Context, name, dir string) {
	if dir == "" {
		return
	}
	if _, err := os.Stat(filepath.Join(dir, "uv")); err != nil {
		return
	}
	_, _ = docker(ctx, time.Minute, "exec", name, "mkdir", "-p", "/root/.local/bin")
	for _, b := range []string{"uv", "uvx"} {
		_, _ = docker(ctx, time.Minute, "cp", filepath.Join(dir, b), name+":/root/.local/bin/"+b)
	}
	envFile := `case ":${PATH}:" in *:"$HOME/.local/bin":*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac`
	_, _ = docker(ctx, time.Minute, "exec", name, "sh", "-c", "printf '%s\\n' '"+envFile+"' > /root/.local/bin/env")
}

// RunTB runs one Terminal-Bench task with qh (or the oracle) and its official verifier.
func RunTB(ctx context.Context, c *llm.Client, h *harness.Harness, t TBTask, rep int, outDir string, o TBOptions, logf func(string, ...any)) (*Result, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	started := time.Now()
	res := &Result{TaskID: "tb-" + t.ID, Tier: "tierC", Rep: rep, HarnessHash: h.Hash, Started: started.UTC().Format(time.RFC3339)}
	if o.Oracle {
		res.HarnessHash = "oracle"
	} else if o.Agent != "" {
		res.HarnessHash = "external"
	}
	fail := func(stage string, err error, out string) (*Result, error) {
		res.Outcome.Status = "error"
		res.Outcome.Err = fmt.Sprintf("%s: %v %s", stage, err, trimStr(out, 500))
		res.Outcome.WallSeconds = time.Since(started).Seconds()
		b, _ := json.MarshalIndent(res, "", "  ")
		_ = os.WriteFile(filepath.Join(outDir, "result.json"), b, 0o644)
		return res, nil
	}
	if out, err := docker(ctx, 20*time.Minute, "pull", "-q", t.Image); err != nil {
		return fail("pull", err, out)
	}
	wd, _ := docker(ctx, time.Minute, "image", "inspect", "-f", "{{.Config.WorkingDir}}", t.Image)
	wd = strings.TrimSpace(wd)
	if wd == "" {
		wd = "/"
	}
	name := fmt.Sprintf("qhtb-%s-r%d-%d", t.ID, rep, time.Now().UnixNano()%1e6)
	args := append([]string{"run", "-d", "--name", name}, containerNetArgs()...)
	if t.CPUs != "" {
		args = append(args, "--cpus", t.CPUs)
	}
	if t.MemoryMB != "" {
		args = append(args, "--memory", t.MemoryMB+"m")
	}
	args = append(args, "--entrypoint", "sh", t.Image, "-c", "sleep infinity")
	if out, err := docker(ctx, 5*time.Minute, args...); err != nil {
		return fail("run", err, out)
	}
	defer func() { _, _ = docker(context.Background(), time.Minute, "rm", "-f", name) }()
	_, _ = docker(ctx, 2*time.Minute, "exec", name, "sh", "-c", netSetup)
	if out, err := docker(ctx, time.Minute, "cp", o.ToolBin, name+":"+tools.RemoteTool); err != nil {
		return fail("cp tool", err, out)
	}
	agentTO := time.Duration(float64(t.AgentTimeout) * o.TimeoutMult)
	actx, cancel := context.WithTimeout(ctx, agentTO+5*time.Minute)
	if o.Oracle {
		_, _ = docker(actx, time.Minute, "cp", filepath.Join(t.Dir, "solution"), name+":/solution")
		out, err := docker(actx, agentTO, "exec", "-w", wd, name, "bash", "/solution/solve.sh")
		_ = os.WriteFile(filepath.Join(outDir, "oracle.log"), []byte(trace.Redact(out)), 0o644)
		res.Outcome.Status = "finished"
		if err != nil {
			res.Outcome.Status = "oracle_failed"
		}
	} else if o.Agent != "" {
		// External agent on the host; it reaches the task only through
		// docker exec on QH_CONTAINER (same isolation as qh's own tools).
		pf, _ := filepath.Abs(filepath.Join(outDir, "prompt.txt"))
		_ = os.WriteFile(pf, []byte(t.Instruction), 0o644)
		env := append(os.Environ(), "QH_CONTAINER="+name, "QH_WORKDIR="+wd, "QH_PROMPT_FILE="+pf,
			fmt.Sprintf("QH_TIMEOUT_S=%d", int(agentTO.Seconds())))
		logf("external agent start")
		out, code, timedOut := tools.Shell(actx, ".", env, o.Agent, agentTO+2*time.Minute)
		_ = os.WriteFile(filepath.Join(outDir, "agent.log"), []byte(trace.Redact(out)), 0o644)
		res.Outcome.Status = "finished"
		if timedOut {
			res.Outcome.Status = "wall_clock"
		} else if code != 0 {
			res.Outcome.Status = fmt.Sprintf("exit_%d", code)
		}
		if endpointDown(ctx, out) {
			cancel()
			return fail("agent", fmt.Errorf("endpoint unavailable (external agent hit an outage)"), "")
		}
	} else {
		tr, err := trace.Open(outDir)
		if err != nil {
			cancel()
			return nil, err
		}
		hh := *h
		hh.Config.MaxWallMinutes = int(agentTO / time.Minute)
		if hh.Config.MaxWallMinutes < 1 {
			hh.Config.MaxWallMinutes = 1
		}
		env := &tools.Env{Container: name, Root: wd, AllowAbs: true}
		ag := loop.New(c, &hh, env, tr, loop.Task{ID: t.ID, Prompt: t.Instruction})
		ag.Log = logf
		res.Outcome = ag.Run(actx)
		tr.Close()
	}
	cancel()
	// Verifier (hidden from the agent and from the harness editor).
	provisionUV(ctx, name, o.UVDir)
	_, _ = docker(ctx, time.Minute, "exec", name, "mkdir", "-p", "/logs/verifier")
	if out, err := docker(ctx, time.Minute, "cp", filepath.Join(t.Dir, "tests"), name+":/tests"); err != nil {
		return fail("cp tests", err, out)
	}
	vout, _ := docker(ctx, t.VerifierTimeout+time.Minute, "exec", "-w", wd, name, "bash", "/tests/test.sh")
	reward, _ := docker(ctx, time.Minute, "exec", name, "cat", "/logs/verifier/reward.txt")
	r, err := strconv.ParseFloat(strings.TrimSpace(reward), 64)
	if err != nil {
		r = 0
		vout += "\n(no reward.txt: " + strings.TrimSpace(reward) + ")"
	}
	res.Score = r
	res.Success = r >= 1
	res.HiddenOut = trace.Redact(tailLines(vout, 60))
	res.Outcome.WallSeconds = time.Since(started).Seconds()
	b, _ := json.MarshalIndent(res, "", "  ")
	return res, os.WriteFile(filepath.Join(outDir, "result.json"), b, 0o644)
}

func trimStr(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
