package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eulercb/little-qwen/internal/harness"
	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/tools"
	"github.com/eulercb/little-qwen/internal/trace"
)

// scripted is one fake model reply.
type scripted struct {
	content   string
	reasoning string
	calls     []llm.ToolCall
	prompt    int // reported prompt_tokens
}

// fakeServer streams scripted replies in order and records requests.
type fakeServer struct {
	mu       sync.Mutex
	replies  []scripted
	requests [][]llm.Message
	efforts  []string
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Messages        []llm.Message `json:"messages"`
		MaxTokens       int           `json:"max_tokens"`
		ReasoningEffort string        `json:"reasoning_effort"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	f.mu.Lock()
	f.requests = append(f.requests, req.Messages)
	f.efforts = append(f.efforts, req.ReasoningEffort)
	var s scripted
	if req.MaxTokens == 1 { // warm-up
		s = scripted{content: "."}
	} else if len(f.replies) > 0 {
		s = f.replies[0]
		f.replies = f.replies[1:]
	} else {
		s = scripted{content: "out of script"}
	}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	delta := map[string]any{}
	if s.reasoning != "" {
		delta["reasoning_content"] = s.reasoning
	}
	if s.content != "" {
		delta["content"] = s.content
	}
	finish := "stop"
	if len(s.calls) > 0 {
		var tcs []map[string]any
		for i, c := range s.calls {
			tcs = append(tcs, map[string]any{"index": i, "id": c.ID, "type": "function", "function": map[string]any{"name": c.Function.Name, "arguments": c.Function.Arguments}})
		}
		delta["tool_calls"] = tcs
		finish = "tool_calls"
	}
	send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta}}})
	send(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}})
	send(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": s.prompt, "completion_tokens": 10}})
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func call(id, name string, args map[string]any) llm.ToolCall {
	b, _ := json.Marshal(args)
	return llm.ToolCall{ID: id, Type: "function", Function: llm.FunctionCall{Name: name, Arguments: string(b)}}
}

func setup(t *testing.T, replies []scripted, cfg func(*harness.Config)) (*Agent, *fakeServer, string) {
	t.Helper()
	fs := &fakeServer{replies: replies}
	srv := httptest.NewServer(fs)
	t.Cleanup(srv.Close)
	hdir := t.TempDir()
	os.WriteFile(filepath.Join(hdir, "systemprompt.md"), []byte("You are a test agent."), 0o644)
	h, err := harness.Load(hdir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		cfg(&h.Config)
	}
	work := t.TempDir()
	c := &llm.Client{BaseURL: srv.URL, Model: "m", HTTP: srv.Client(), IdleTimeout: 5 * time.Second, MaxWait: time.Second, Log: func(string, ...any) {}}
	tr, err := trace.Open(filepath.Join(work, ".qh", "trace"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tr.Close)
	a := New(c, h, &tools.Env{Root: work}, tr, Task{ID: "t", Prompt: "Make hello.txt", Check: "test -f hello.txt"})
	return a, fs, work
}

func TestLoopWriteThenFinish(t *testing.T) {
	a, fs, work := setup(t, []scripted{
		{calls: []llm.ToolCall{call("c1", "finish", map[string]any{"summary": "done"})}, prompt: 100},
		{calls: []llm.ToolCall{call("c2", "write_file", map[string]any{"path": "hello.txt", "content": "hi\n"})}, prompt: 200},
		{calls: []llm.ToolCall{call("c3", "finish", map[string]any{"summary": "wrote it"})}, prompt: 300},
	}, nil)
	o := a.Run(context.Background())
	if o.Status != "finished" || !o.CheckPassed || o.FinishAttempts != 2 {
		t.Fatalf("%+v", o)
	}
	if _, err := os.Stat(filepath.Join(work, "hello.txt")); err != nil {
		t.Fatal(err)
	}
	// Every request after the first is an append-only extension of the previous one.
	var prev []llm.Message
	for i, req := range fs.requests {
		if len(req) == 2 && req[1].Text() == "." {
			continue // warm-up
		}
		if prev != nil {
			for j := range prev {
				a, _ := json.Marshal(prev[j])
				b, _ := json.Marshal(req[j])
				if string(a) != string(b) {
					t.Fatalf("request %d rewrote message %d", i, j)
				}
			}
		}
		prev = req
	}
}

func TestLoopRepairsLeakedXMLCall(t *testing.T) {
	xml := "I will write it.\n<tool_call>\n<function=write_file>\n<parameter=path>\nhello.txt\n</parameter>\n<parameter=content>\nhi\n</parameter>\n</function>\n</tool_call>"
	a, _, work := setup(t, []scripted{
		{content: xml, prompt: 100},
		{calls: []llm.ToolCall{call("c3", "finish", map[string]any{"summary": "ok"})}, prompt: 300},
	}, nil)
	o := a.Run(context.Background())
	if !o.CheckPassed || o.Repaired != 1 {
		t.Fatalf("%+v", o)
	}
	b, _ := os.ReadFile(filepath.Join(work, "hello.txt"))
	if string(b) != "hi" {
		t.Fatalf("content %q", b)
	}
}

func TestLoopNudgesThenGivesUp(t *testing.T) {
	a, fs, _ := setup(t, []scripted{
		{content: "thinking about it", prompt: 100},
		{content: "still thinking", prompt: 120},
		{content: "hmm", prompt: 140},
	}, func(c *harness.Config) { c.MaxNudges = 2 })
	o := a.Run(context.Background())
	if o.Status != "gave_up" || o.Nudges != 3 {
		t.Fatalf("%+v", o)
	}
	last := fs.requests[len(fs.requests)-1]
	if !strings.Contains(last[len(last)-1].Text(), "without calling a tool") {
		t.Fatalf("no nudge: %q", last[len(last)-1].Text())
	}
}

func TestLoopCompactsAndKeepsEffort(t *testing.T) {
	a, fs, work := setup(t, []scripted{
		{calls: []llm.ToolCall{call("c1", "list_dir", map[string]any{"path": "."})}, prompt: 9500},
		{content: "## Progress\nnothing yet\n## Next steps\nwrite hello.txt", prompt: 9600}, // checkpoint
		{calls: []llm.ToolCall{call("c2", "write_file", map[string]any{"path": "hello.txt", "content": "x"})}, prompt: 3000},
		{calls: []llm.ToolCall{call("c3", "finish", map[string]any{"summary": "ok"})}, prompt: 3100},
	}, func(c *harness.Config) {
		c.CompactAtTokens = 9000
		c.ReasoningEffort = "medium"
		c.PlannerEffort = "medium"
	})
	o := a.Run(context.Background())
	if !o.CheckPassed || o.Compactions != 1 {
		t.Fatalf("%+v", o)
	}
	for i, e := range fs.efforts {
		if e != "medium" {
			t.Fatalf("request %d used effort %q (must stay constant to keep the cache)", i, e)
		}
	}
	// After compaction the context restarts: system + task card with checkpoint.
	var post []llm.Message
	for _, r := range fs.requests {
		if len(r) >= 2 && strings.Contains(r[1].Text(), "# Checkpoint 1") {
			post = r
			break
		}
	}
	if post == nil || !strings.Contains(post[1].Text(), "write hello.txt") {
		t.Fatal("checkpoint not carried into the new segment")
	}
	if _, err := os.Stat(filepath.Join(work, ".qh", "state", "CHECKPOINT-1.md")); err != nil {
		t.Fatal(err)
	}
}

func TestSubAgentReadOnly(t *testing.T) {
	a, _, work := setup(t, []scripted{
		{calls: []llm.ToolCall{call("d1", "delegate", map[string]any{"task": "find things"})}, prompt: 100},
		// sub-agent turns:
		{calls: []llm.ToolCall{call("s1", "write_file", map[string]any{"path": "evil.txt", "content": "x"})}, prompt: 50},
		{calls: []llm.ToolCall{call("s2", "finish", map[string]any{"summary": "found: nothing"})}, prompt: 60},
		// main resumes:
		{calls: []llm.ToolCall{call("c2", "write_file", map[string]any{"path": "hello.txt", "content": "x"})}, prompt: 200},
		{calls: []llm.ToolCall{call("c3", "finish", map[string]any{"summary": "ok"})}, prompt: 300},
	}, nil)
	o := a.Run(context.Background())
	if !o.CheckPassed || o.Delegations != 1 {
		t.Fatalf("%+v", o)
	}
	if _, err := os.Stat(filepath.Join(work, "evil.txt")); err == nil {
		t.Fatal("sub-agent wrote a file")
	}
}
