package eval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadResults finds every result.json under dir.
func LoadResults(dir string) ([]Result, error) {
	var out []Result
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && info.Name() == ".inprogress" {
			return filepath.SkipDir
		}
		if info.Name() == "result.json" {
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			var r Result
			if json.Unmarshal(b, &r) == nil {
				out = append(out, r)
			}
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return out[i].TaskID < out[j].TaskID
		}
		return out[i].Rep < out[j].Rep
	})
	return out, err
}

func meanSD(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	m := 0.0
	for _, x := range xs {
		m += x
	}
	m /= float64(len(xs))
	v := 0.0
	for _, x := range xs {
		v += (x - m) * (x - m)
	}
	if len(xs) > 1 {
		v /= float64(len(xs) - 1)
	}
	return m, math.Sqrt(v)
}

// Summary renders a markdown report for a results directory.
func Summary(results []Result) string {
	var b strings.Builder
	if len(results) == 0 {
		return "no results\n"
	}
	byTask := map[string][]Result{}
	var ids []string
	codes := map[string]int{}
	var succ, score, wall, turns, prefill, reprefill, comp, think []float64
	toolCalls, toolErr, repaired := 0, 0, 0
	hashes := map[string]bool{}
	for _, r := range results {
		if _, ok := byTask[r.TaskID]; !ok {
			ids = append(ids, r.TaskID)
		}
		byTask[r.TaskID] = append(byTask[r.TaskID], r)
		s := 0.0
		if r.Success {
			s = 1
		}
		succ = append(succ, s)
		score = append(score, r.Score)
		o := r.Outcome
		wall = append(wall, o.WallSeconds/60)
		turns = append(turns, float64(o.Turns))
		prefill = append(prefill, float64(o.PrefillTokens))
		reprefill = append(reprefill, float64(o.ReprefillTokens))
		comp = append(comp, float64(o.CompletionTokens))
		think = append(think, float64(o.ReasoningTokens))
		toolCalls += o.ToolCalls
		toolErr += o.ToolErrors
		repaired += o.Repaired
		for c, n := range o.Codes {
			codes[c] += n
		}
		hashes[r.HarnessHash] = true
	}
	sort.Strings(ids)
	var hs []string
	for h := range hashes {
		hs = append(hs, h)
	}
	sort.Strings(hs)
	f := func(xs []float64, prec int) string {
		m, sd := meanSD(xs)
		return fmt.Sprintf("%.*f ± %.*f", prec, m, prec, sd)
	}
	fmt.Fprintf(&b, "Harness: %s — %d runs over %d tasks\n\n", strings.Join(hs, ","), len(results), len(ids))
	fmt.Fprintf(&b, "| metric | mean ± sd |\n|---|---|\n")
	fmt.Fprintf(&b, "| success | %s |\n| score | %s |\n| wall (min) | %s |\n| turns | %s |\n", f(succ, 2), f(score, 2), f(wall, 1), f(turns, 1))
	fmt.Fprintf(&b, "| prefill tokens | %s |\n| re-prefill tokens | %s |\n| completion tokens | %s |\n| reasoning tokens | %s |\n", f(prefill, 0), f(reprefill, 0), f(comp, 0), f(think, 0))
	er := 0.0
	if toolCalls > 0 {
		er = float64(toolErr) / float64(toolCalls)
	}
	fmt.Fprintf(&b, "| tool calls / errors / repaired | %d / %d (%.1f%%) / %d |\n\n", toolCalls, toolErr, 100*er, repaired)
	var cs []string
	for c := range codes {
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool { return codes[cs[i]] > codes[cs[j]] })
	b.WriteString("Failure codes: ")
	for _, c := range cs {
		fmt.Fprintf(&b, "%s=%d ", c, codes[c])
	}
	b.WriteString("\n\n| task | tier | success | score | status | turns | wall min | compactions |\n|---|---|---|---|---|---|---|---|\n")
	for _, id := range ids {
		rs := byTask[id]
		var sc, sx, wl, tn []float64
		var st []string
		comps := 0
		for _, r := range rs {
			s := 0.0
			if r.Success {
				s = 1
			}
			sx = append(sx, s)
			sc = append(sc, r.Score)
			wl = append(wl, r.Outcome.WallSeconds/60)
			tn = append(tn, float64(r.Outcome.Turns))
			st = append(st, r.Outcome.Status)
			comps += r.Outcome.Compactions
		}
		ms, _ := meanSD(sx)
		mc, _ := meanSD(sc)
		mw, _ := meanSD(wl)
		mt, _ := meanSD(tn)
		fmt.Fprintf(&b, "| %s | %s | %.2f | %.2f | %s | %.0f | %.1f | %d |\n", id, rs[0].Tier, ms, mc, strings.Join(st, ","), mt, mw, comps)
	}
	return b.String()
}

// Digest renders an editor-facing digest of one run directory: turn list,
// tool calls, errors and events. It never includes hidden check output.
func Digest(runDir string, maxLines int) (string, error) { return DigestWith(runDir, maxLines, 0) }

// turnThoughts maps (agent, turn index) to a short excerpt of the assistant's
// reasoning and text for that turn, read from transcript.jsonl.
func turnThoughts(runDir string, n int) map[string]string {
	out := map[string]string{}
	f, err := os.Open(filepath.Join(runDir, "transcript.jsonl"))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	turn := map[string]int{}
	for sc.Scan() {
		var rec struct {
			Agent string `json:"agent"`
			Msg   struct {
				Role      string  `json:"role"`
				Content   *string `json:"content"`
				Reasoning string  `json:"reasoning_content"`
			} `json:"msg"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) != nil || rec.Msg.Role != "assistant" {
			continue
		}
		turn[rec.Agent]++
		txt := rec.Msg.Reasoning
		if rec.Msg.Content != nil && strings.TrimSpace(*rec.Msg.Content) != "" {
			txt += " | said: " + *rec.Msg.Content
		}
		out[fmt.Sprintf("%s/%d", rec.Agent, turn[rec.Agent])] = oneLineStr(txt, n)
	}
	return out
}

// DigestWith is Digest plus, when thoughtChars > 0, an excerpt of each turn's
// reasoning. Turn numbers in the transcript count assistant messages, which
// can drift from trace turns after compaction checkpoints; excerpts are
// best-effort context.
func DigestWith(runDir string, maxLines, thoughtChars int) (string, error) {
	f, err := os.Open(filepath.Join(runDir, "trace.jsonl"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	var b strings.Builder
	if rb, err := os.ReadFile(filepath.Join(runDir, "result.json")); err == nil {
		var r Result
		if json.Unmarshal(rb, &r) == nil {
			o := r.Outcome
			fmt.Fprintf(&b, "# %s rep %d — status=%s turns=%d compactions=%d wall=%.1fmin\n", r.TaskID, r.Rep, o.Status, o.Turns, o.Compactions, o.WallSeconds/60)
			fmt.Fprintf(&b, "prefill=%d reprefill=%d completion=%d reasoning=%d tool_calls=%d tool_errors=%d codes=%v\n", o.PrefillTokens, o.ReprefillTokens, o.CompletionTokens, o.ReasoningTokens, o.ToolCalls, o.ToolErrors, o.Codes)
			if o.Summary != "" {
				fmt.Fprintf(&b, "finish summary: %s\n", oneLineStr(o.Summary, 400))
			}
			b.WriteString("\n")
		}
	}
	var thoughts map[string]string
	if thoughtChars > 0 {
		thoughts = turnThoughts(runDir, thoughtChars)
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	n := 0
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) != nil {
			continue
		}
		var line string
		switch m["type"] {
		case "turn":
			if thoughtChars > 0 {
				if th := thoughts[fmt.Sprintf("%v/%v", m["agent"], m["turn"])]; th != "" {
					b.WriteString(fmt.Sprintf("  ~ %s\n", th))
				}
			}
			line = fmt.Sprintf("T%v %v prompt=%v cached=%v out=%v think=%v %.0fs %v", m["turn"], m["agent"], m["prompt_tokens"], m["cached_tokens"], m["completion_tokens"], m["reasoning_tokens"], toF(m["latency_ms"])/1000, m["finish_reason"])
			if toF(m["reprefill_tokens"]) > 0 {
				line += fmt.Sprintf(" REPREFILL=%v", m["reprefill_tokens"])
			}
		case "tool":
			st := "ok"
			if m["ok"] != true {
				st = fmt.Sprintf("ERR %v", m["error_class"])
			}
			line = fmt.Sprintf("    %v(%v) -> %s", m["tool"], oneLineStr(fmt.Sprint(m["args_preview"]), 140), st)
			if m["test_signal"] != nil {
				line += fmt.Sprintf(" test=%v", m["test_signal"])
			}
			if m["spilled"] == true {
				line += " SPILLED"
			}
		case "event":
			line = fmt.Sprintf("  ! %v %v %v", m["name"], m["code"], oneLineStr(fmt.Sprint(m["detail"]), 200))
		}
		if line == "" {
			continue
		}
		b.WriteString(line + "\n")
		n++
		if maxLines > 0 && n >= maxLines {
			b.WriteString("... (truncated)\n")
			break
		}
	}
	return b.String(), nil
}

func toF(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func oneLineStr(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
