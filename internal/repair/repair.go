// Package repair recovers tool calls the server failed to parse (they leak
// into content as Qwen3-Coder XML or Hermes JSON) and detects loops.
package repair

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/eulercb/little-qwen/internal/llm"
)

var (
	toolCallBlock = regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*(?:</tool_call>|$)`)
	funcBlock     = regexp.MustCompile(`(?s)<function=([^>\s]+)>\s*(.*?)\s*(?:</function>|$)`)
	paramBlock    = regexp.MustCompile(`(?s)<parameter=([^>\s]+)>\n?(.*?)\n?</parameter>`)
)

// LooksLikeToolCall reports whether content contains tool-call markup.
func LooksLikeToolCall(content string) bool {
	return strings.Contains(content, "<tool_call>") || strings.Contains(content, "<function=")
}

// ParseContent extracts tool calls from raw content. It accepts the
// Qwen3-Coder XML format and the Hermes JSON format.
func ParseContent(content string) []llm.ToolCall {
	var calls []llm.ToolCall
	bodies := []string{}
	for _, m := range toolCallBlock.FindAllStringSubmatch(content, -1) {
		bodies = append(bodies, m[1])
	}
	if len(bodies) == 0 && strings.Contains(content, "<function=") {
		bodies = append(bodies, content)
	}
	for i, body := range bodies {
		if fm := funcBlock.FindAllStringSubmatch(body, -1); len(fm) > 0 {
			for _, f := range fm {
				args := map[string]any{}
				for _, p := range paramBlock.FindAllStringSubmatch(f[2], -1) {
					args[p[1]] = coerce(p[2])
				}
				b, _ := json.Marshal(args)
				calls = append(calls, llm.ToolCall{ID: fmt.Sprintf("repaired_%d_%d", i, len(calls)), Type: "function",
					Function: llm.FunctionCall{Name: f[1], Arguments: string(b)}})
			}
			continue
		}
		var j struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &j); err == nil && j.Name != "" {
			args := string(j.Arguments)
			var s string
			if json.Unmarshal(j.Arguments, &s) == nil {
				args = s
			}
			if args == "" {
				args = "{}"
			}
			calls = append(calls, llm.ToolCall{ID: fmt.Sprintf("repaired_%d_%d", i, len(calls)), Type: "function",
				Function: llm.FunctionCall{Name: j.Name, Arguments: args}})
		}
	}
	return calls
}

// coerce keeps parameter values as strings unless they are plain integers.
func coerce(v string) any {
	t := strings.TrimSpace(v)
	if t != "" && len(t) < 12 && strings.Trim(t, "0123456789-") == "" {
		var n int
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			return n
		}
	}
	return v
}

// RepairArgs tries to fix malformed JSON arguments (truncated or with raw
// newlines). It returns the input unchanged when it cannot help.
func RepairArgs(raw string) (string, bool) {
	var v map[string]any
	if json.Unmarshal([]byte(raw), &v) == nil {
		return raw, false
	}
	s := strings.TrimSpace(raw)
	// Raw control characters inside strings.
	fixed := escapeControlInStrings(s)
	if json.Unmarshal([]byte(fixed), &v) == nil {
		return fixed, true
	}
	// Missing closing brace/quote.
	for _, suf := range []string{"}", "\"}", "\"\n}"} {
		if json.Unmarshal([]byte(fixed+suf), &v) == nil {
			return fixed + suf, true
		}
	}
	return raw, false
}

func escapeControlInStrings(s string) string {
	var b strings.Builder
	in := false
	esc := false
	for _, r := range s {
		if in {
			if esc {
				esc = false
				b.WriteRune(r)
				continue
			}
			switch r {
			case '\\':
				esc = true
				b.WriteRune(r)
			case '"':
				in = false
				b.WriteRune(r)
			case '\n':
				b.WriteString(`\n`)
			case '\t':
				b.WriteString(`\t`)
			case '\r':
				b.WriteString(`\r`)
			default:
				b.WriteRune(r)
			}
			continue
		}
		if r == '"' {
			in = true
		}
		b.WriteRune(r)
	}
	return b.String()
}

// LoopDetector flags repeated identical calls and long stretches of
// activity without progress (no file change, no new test result).
type LoopDetector struct {
	Window     int // look-back window for identical calls
	MaxRepeat  int // identical calls within Window that trigger a hint
	MaxNoProg  int // calls without progress that trigger a hint
	recent     []string
	noProgress int
}

func NewLoopDetector() *LoopDetector {
	return &LoopDetector{Window: 8, MaxRepeat: 3, MaxNoProg: 14}
}

// Hash identifies a call by tool + canonical args.
func Hash(name, args string) string {
	var v any
	if json.Unmarshal([]byte(args), &v) == nil {
		if b, err := json.Marshal(v); err == nil {
			args = string(b)
		}
	}
	h := sha1.Sum([]byte(name + "\x00" + args))
	return hex.EncodeToString(h[:8])
}

// Observe records a call; progress is true when it changed a file or
// produced a new test signal. It returns a non-empty hint when a loop is
// detected.
func (d *LoopDetector) Observe(name, args string, progress bool) (hint string, class string) {
	h := Hash(name, args)
	d.recent = append(d.recent, h)
	if len(d.recent) > d.Window {
		d.recent = d.recent[len(d.recent)-d.Window:]
	}
	if progress {
		d.noProgress = 0
	} else {
		d.noProgress++
	}
	n := 0
	for _, r := range d.recent {
		if r == h {
			n++
		}
	}
	if n >= d.MaxRepeat {
		d.recent = nil
		return fmt.Sprintf("LOOP WARNING: you have made this exact %s call %d times recently and got the same result. Stop repeating it. State in one sentence what you learned, then choose a different action.", name, n), "repeat"
	}
	if d.noProgress >= d.MaxNoProg {
		d.noProgress = 0
		return fmt.Sprintf("PROGRESS WARNING: %d tool calls without changing any file or running a build/test. Stop exploring: write down your hypothesis, make the smallest concrete change, then run the tests.", d.MaxNoProg), "no_progress"
	}
	return "", ""
}
