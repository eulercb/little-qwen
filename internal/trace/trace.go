// Package trace writes per-run JSONL traces and the full message transcript.
// Traces are the evidence a harness editor reads (as digests) when proposing
// changes, so every field here should be cheap to aggregate.
package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Failure taxonomy codes (see docs/BUILD_PLAN.md §3.8).
const (
	F1ToolFormat    = "F1"  // tool-format error / repaired call
	F2WrongTool     = "F2"  // wrong tool or args
	F3Loop          = "F3"  // loop
	F4ThinkOverflow = "F4"  // hit max_tokens while thinking
	F5Context       = "F5"  // context overflow / compaction loss
	F6ClaimedDone   = "F6"  // finish claimed but acceptance failed
	F7Spec          = "F7"  // spec misunderstanding (assigned by review)
	F8Infra         = "F8"  // endpoint/environment
	F9Regression    = "F9"  // broke an earlier step (assigned by review)
	F10GaveUp       = "F10" // gave up / empty deliverable / budget exhausted
)

type Turn struct {
	Type             string `json:"type"` // "turn"
	T                string `json:"t"`
	Agent            string `json:"agent"`
	Segment          int    `json:"segment"` // increments at each compaction
	Turn             int    `json:"turn"`
	PromptTokens     int    `json:"prompt_tokens"`
	CachedTokens     int    `json:"cached_tokens"`
	PrefillTokens    int    `json:"prefill_tokens"`
	ReprefillTokens  int    `json:"reprefill_tokens"` // previous prompt tokens the cache failed to reuse
	CompletionTokens int    `json:"completion_tokens"`
	ReasoningTokens  int    `json:"reasoning_tokens"`
	LatencyMS        int64  `json:"latency_ms"`
	TTFTMS           int64  `json:"ttft_ms"`
	FinishReason     string `json:"finish_reason"`
	CacheStatus      string `json:"cache_status"`
	ToolCalls        int    `json:"tool_calls"`
	Repaired         bool   `json:"repaired,omitempty"`
	Retries          int    `json:"retries,omitempty"`
	Effort           string `json:"effort"`
}

type Tool struct {
	Type        string   `json:"type"` // "tool"
	T           string   `json:"t"`
	Agent       string   `json:"agent"`
	Turn        int      `json:"turn"`
	Tool        string   `json:"tool"`
	ArgsHash    string   `json:"args_hash"`
	ArgsPreview string   `json:"args_preview"`
	OK          bool     `json:"ok"`
	ErrorClass  string   `json:"error_class,omitempty"`
	OutChars    int      `json:"out_chars"`
	Spilled     bool     `json:"spilled,omitempty"`
	Changed     []string `json:"changed,omitempty"`
	TestSignal  string   `json:"test_signal,omitempty"`
	DurMS       int64    `json:"dur_ms"`
}

type Event struct {
	Type   string         `json:"type"` // "event"
	T      string         `json:"t"`
	Agent  string         `json:"agent"`
	Turn   int            `json:"turn"`
	Name   string         `json:"name"`
	Code   string         `json:"code,omitempty"` // failure code if any
	Detail string         `json:"detail,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

// Tracer appends to <dir>/trace.jsonl and <dir>/transcript.jsonl.
type Tracer struct {
	mu         sync.Mutex
	trace      *os.File
	transcript *os.File
	Dir        string
}

func Open(dir string) (*Tracer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	t, err := os.OpenFile(filepath.Join(dir, "trace.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	tr, err := os.OpenFile(filepath.Join(dir, "transcript.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Tracer{trace: t, transcript: tr, Dir: dir}, nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func (t *Tracer) write(f *os.File, v any) {
	if t == nil {
		return
	}
	b, _ := json.Marshal(v)
	b = []byte(Redact(string(b)))
	t.mu.Lock()
	defer t.mu.Unlock()
	f.Write(append(b, '\n'))
}

func (t *Tracer) Turn(v Turn) {
	v.Type, v.T = "turn", now()
	t.write(t.trace, v)
}

func (t *Tracer) Tool(v Tool) {
	v.Type, v.T = "tool", now()
	t.write(t.trace, v)
}

func (t *Tracer) Event(v Event) {
	v.Type, v.T = "event", now()
	t.write(t.trace, v)
}

// Message records one message as it enters an agent's context.
func (t *Tracer) Message(agent string, segment int, msg any) {
	t.write(t.transcript, map[string]any{"agent": agent, "segment": segment, "msg": msg})
}

func (t *Tracer) Close() {
	if t == nil {
		return
	}
	t.trace.Close()
	t.transcript.Close()
}
