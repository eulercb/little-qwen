package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// RemoteTool is the path of the static qh binary inside task containers.
const RemoteTool = "/usr/local/bin/qh-tool"

// wireResult is the JSON exchanged with `qh toolexec`.
type wireResult struct {
	OK         bool     `json:"ok"`
	ErrorClass string   `json:"error_class,omitempty"`
	Output     string   `json:"output"`
	Changed    []string `json:"changed,omitempty"`
	TestSignal string   `json:"test_signal,omitempty"`
}

type wireRequest struct {
	Root    string `json:"root"`
	Tool    string `json:"tool"`
	Args    Args   `json:"args"`
	Fuzzy   bool   `json:"fuzzy"`
	LineSep string `json:"line_sep"`
}

// remoteExec runs one tool inside e.Container.
func (e *Env) remoteExec(ctx context.Context, name string, a Args) Result {
	req, _ := json.Marshal(wireRequest{Root: e.Root, Tool: name, Args: a, Fuzzy: e.FuzzyEdit, LineSep: e.LineSep})
	to := 2 * time.Minute
	if name == "run" {
		to = time.Duration(a.Int("timeout_s", 120)+30) * time.Second
		if max := e.MaxRun + 30*time.Second; e.MaxRun > 0 && to > max {
			to = max
		}
		if to > 11*time.Minute {
			to = 11 * time.Minute
		}
	}
	cctx, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	cmd := exec.CommandContext(cctx, "docker", "exec", "-i", "-w", e.Root, e.Container, RemoteTool, "toolexec")
	cmd.Stdin = bytes.NewReader(req)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		return fail("container", "docker exec failed: %v %s", err, trimTo(errb.String(), 300))
	}
	var w wireResult
	if err := json.Unmarshal(out.Bytes(), &w); err != nil {
		return fail("container", "bad tool reply: %v %s", err, trimTo(out.String()+errb.String(), 300))
	}
	return Result{OK: w.OK, ErrorClass: w.ErrorClass, Output: w.Output, Changed: w.Changed, TestSignal: w.TestSignal}
}

// ToolExecMain implements `qh toolexec` inside a container: read a
// wireRequest on stdin, run the tool locally, print a wireResult.
func ToolExecMain(stdin io.Reader, stdout io.Writer) error {
	b, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	var req wireRequest
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}
	env := &Env{Root: req.Root, RunEnv: os.Environ(), AllowAbs: true, FuzzyEdit: req.Fuzzy, LineSep: req.LineSep}
	r := env.Exec(context.Background(), req.Tool, req.Args)
	return json.NewEncoder(stdout).Encode(wireResult{OK: r.OK, ErrorClass: r.ErrorClass, Output: r.Output, Changed: r.Changed, TestSignal: r.TestSignal})
}

// WriteFile writes data at path in the agent's filesystem (host or container).
func (e *Env) WriteFile(path string, data []byte) error {
	if e.Container == "" {
		return os.WriteFile(path, data, 0o644)
	}
	r := e.Exec(context.Background(), "write_file", Args{"path": path, "content": string(data)})
	if !r.OK {
		return fmt.Errorf("%s", r.Output)
	}
	return nil
}

// RunCmd runs a shell command in the agent's environment (host or container)
// and returns output and exit code.
func (e *Env) RunCmd(ctx context.Context, cmdline string, to time.Duration) (string, int) {
	if e.Container == "" {
		out, code, _ := Shell(ctx, e.Root, e.RunEnv, cmdline, to)
		return out, code
	}
	r := e.Exec(ctx, "run", Args{"command": cmdline, "timeout_s": float64(to / time.Second)})
	// Strip the status line added by the run tool.
	out := r.Output
	if i := bytes.IndexByte([]byte(out), '\n'); i >= 0 {
		out = out[i+1:]
	}
	if r.OK {
		return out, 0
	}
	return out, 1
}

func trimTo(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
