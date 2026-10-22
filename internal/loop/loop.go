// Package loop is the executor agent loop. Its central rule is prefix
// stability: between compactions the message list is append-only, so every
// request is a strict extension of the previous one and the server's prefix
// cache covers everything but the new tail.
package loop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eulercb/little-qwen/internal/ctxmgr"
	"github.com/eulercb/little-qwen/internal/harness"
	"github.com/eulercb/little-qwen/internal/llm"
	"github.com/eulercb/little-qwen/internal/repair"
	"github.com/eulercb/little-qwen/internal/tools"
	"github.com/eulercb/little-qwen/internal/trace"
)

// Task is what the executor is asked to do.
type Task struct {
	ID     string
	Prompt string // goal + acceptance criteria, markdown
	Check  string // visible acceptance command run by the harness on finish ("" = none)
}

// Outcome summarises a run.
type Outcome struct {
	Status           string // finished | check_failed | max_turns | wall_clock | gave_up | error
	CheckPassed      bool
	Summary          string
	Turns            int
	Compactions      int
	PromptTokens     int // sum over turns
	PrefillTokens    int // sum over turns (actually computed)
	ReprefillTokens  int
	CompletionTokens int
	ReasoningTokens  int
	ToolCalls        int
	ToolErrors       int
	Repaired         int
	Nudges           int
	LoopHints        int
	FinishAttempts   int
	Delegations      int
	Codes            map[string]int
	WallSeconds      float64
	Err              string
}

// Agent runs one task (or one delegated sub-task).
type Agent struct {
	Name    string
	Client  *llm.Client
	H       *harness.Harness
	Env     *tools.Env
	Tr      *trace.Tracer
	Task    Task
	Log     func(format string, args ...any)
	Tools   []llm.Tool
	IsSub   bool
	MaxTurn int

	cfg         harness.Config
	msgs        []llm.Message
	segment     int
	lastProm    int
	loops       *repair.LoopDetector
	out         Outcome
	finishOK    bool
	subN        int
	nudges      int
	errSeen     map[string]int
	start       time.Time
	reviewed    bool
	allowed     map[string]bool // nil = all tools allowed
	overBudget  bool            // previous turn hit the thinking budget
	noThinkNext bool            // run the next request with thinking off (think_cap_continue)
	lastTurnBad bool            // previous turn had a tool error, failed test/check or no tool call
	prevProm    int             // prompt tokens of the previous request (for cache-loss accounting)
}

// New creates the top-level agent.
func New(c *llm.Client, h *harness.Harness, env *tools.Env, tr *trace.Tracer, task Task) *Agent {
	env.LineSep = h.Config.ReadLineSep
	env.FuzzyEdit = h.Config.FuzzyEdit
	return &Agent{Name: "main", Client: c, H: h, Env: env, Tr: tr, Task: task, Tools: tools.Definitions(nil), MaxTurn: h.Config.MaxTurns}
}

func (a *Agent) logf(format string, args ...any) {
	if a.Log != nil {
		a.Log("[%s] "+format, append([]any{a.Name}, args...)...)
	}
}

func (a *Agent) push(m llm.Message) {
	a.Tr.Message(a.Name, a.segment, m)
	if a.H.Config.StripReasoning {
		// Never send reasoning back. Applied to every assistant message from
		// the start, so the prefix stays stable and thinking is neither
		// re-prefilled nor kept in context.
		m.ReasoningContent = ""
	}
	a.msgs = append(a.msgs, m)
}

func (a *Agent) code(c string) {
	if a.out.Codes == nil {
		a.out.Codes = map[string]int{}
	}
	a.out.Codes[c]++
}

// TaskCard renders the first user message. It is written once per context
// segment and never edited afterwards.
func (a *Agent) TaskCard() string {
	var b strings.Builder
	if a.IsSub {
		b.WriteString("# Delegated sub-task (read-only)\n")
		b.WriteString("You are a sub-agent with a fresh context. Investigate and answer the question below. Do NOT modify files. When done, call finish with a concise report (at most ~300 words): concrete file paths, line numbers, names and facts. The caller only sees your finish summary.\n\n")
		if s, ok := a.H.SubAgents["explorer"]; ok {
			b.WriteString(strings.TrimSpace(s) + "\n\n")
		}
		b.WriteString("## Question\n" + a.Task.Prompt + "\n")
		return b.String()
	}
	b.WriteString("# Task\n")
	b.WriteString(strings.TrimSpace(a.Task.Prompt) + "\n\n")
	b.WriteString("# Workspace\n")
	if a.Env.Container != "" {
		fmt.Fprintf(&b, "You are root inside a disposable Linux container. Tools run in the container; relative paths are relative to %s, and absolute paths anywhere are allowed. You may install packages (internet access is available). Nobody will check your work until you call finish, and then it is graded automatically, so verify it yourself first.\n", a.Env.Root)
	} else {
		fmt.Fprintf(&b, "Your workspace root is the current directory for all tools (absolute path: %s). It is a git repository; the initial state is committed.\n", a.Env.Root)
	}
	if a.Task.Check != "" {
		fmt.Fprintf(&b, "\n# Acceptance check\nWhen you call finish, the harness runs: `%s`\nIt must exit 0. Run it yourself before calling finish.\n", a.Task.Check)
	}
	listing := a.Env.Exec(context.Background(), "list_dir", tools.Args{"path": ".", "depth": float64(3)})
	out, _ := ctxmgr.Cap(a.Env, "initial-listing", listing.Output, 1200)
	b.WriteString("\n# Initial file tree\n```\n" + strings.TrimPrefix(out, "OK ") + "```\n")
	return b.String()
}

// Run executes the loop until finish, budget exhaustion or error.
func (a *Agent) Run(ctx context.Context) Outcome {
	start := time.Now()
	a.cfg = a.H.Config
	if a.MaxTurn == 0 {
		a.MaxTurn = a.cfg.MaxTurns
	}
	a.loops = repair.NewLoopDetector()
	if a.cfg.LoopRepeat > 0 {
		a.loops.MaxRepeat = a.cfg.LoopRepeat
	}
	if a.cfg.LoopNoProgress > 0 {
		a.loops.MaxNoProg = a.cfg.LoopNoProgress
	}
	a.errSeen = map[string]int{}
	wall := time.Duration(a.cfg.MaxWallMinutes) * time.Minute
	if a.IsSub {
		wall = wall / 3
	}
	deadline := start.Add(wall)

	a.push(llm.Message{Role: "system", Content: llm.Str(a.H.System)})
	a.warmPrefix(ctx)
	a.push(llm.Message{Role: "user", Content: llm.Str(a.TaskCard())})

	a.start = start
	for turn := 1; ; turn++ {
		if turn > a.MaxTurn {
			a.out.Status = "max_turns"
			a.code(trace.F10GaveUp)
			return a.finalize()
		}
		if time.Now().After(deadline) {
			a.out.Status = "wall_clock"
			a.code(trace.F10GaveUp)
			return a.finalize()
		}
		if ctx.Err() != nil {
			a.out.Status, a.out.Err = "error", ctx.Err().Error()
			return a.finalize()
		}
		if a.lastProm > a.cfg.CompactAtTokens && !a.IsSub {
			if err := a.compact(ctx, turn, "threshold"); err != nil {
				a.out.Status, a.out.Err = "error", err.Error()
				a.code(trace.F8Infra)
				return a.finalize()
			}
		}
		a.out.Turns = turn
		effort := a.effort(turn)
		thinkChars := 0
		if a.cfg.ThinkBudgetTokens > 0 && !a.overBudget {
			thinkChars = int(float64(a.cfg.ThinkBudgetTokens) * 3.5)
		}
		req := llm.Request{
			Messages: a.msgs, Tools: a.Tools, MaxTokens: a.cfg.MaxTokens, ReasoningEffort: effort,
			Temperature: a.cfg.Temperature, TopP: a.cfg.TopP, PresencePenalty: a.cfg.PresencePenalty,
			MaxThinkChars: thinkChars,
		}
		if !a.thinkThisTurn(turn) || a.noThinkNext {
			// Thinking on/off only changes the generation-prompt suffix, so it
			// does not disturb the cached prefix (docs/ENDPOINT.md). Use Qwen's
			// recommended non-thinking sampling.
			req.DisableThinking = true
			t, p, pp := 0.7, 0.8, 1.5
			req.Temperature, req.TopP, req.PresencePenalty = &t, &p, &pp
		}
		resp, err := a.Client.Chat(ctx, req)
		a.noThinkNext = false
		if errors.Is(err, llm.ErrThinkBudget) {
			// Runaway thinking: drop the partial turn (never sent back, so the
			// prefix is unchanged) and ask for a decision. The next turn runs
			// without the budget so a genuinely hard step can still finish.
			a.overBudget = true
			a.code(trace.F4ThinkOverflow)
			a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "think_budget", Code: trace.F4ThinkOverflow,
				Data: map[string]any{"reasoning_chars": len(resp.Message.ReasoningContent), "latency_ms": resp.LatencyMS}})
			a.logf("turn %d: thinking budget exceeded after %.0fs; asking for a decision", turn, float64(resp.LatencyMS)/1000)
			if a.cfg.ThinkCapContinue {
				// Keep the partial reasoning (EXP-3 showed that discarding it
				// only moves the work) and make the next turn act on it with
				// thinking off; thinking on/off is cache-neutral.
				a.push(llm.Message{Role: "assistant", Content: llm.Str(""), ReasoningContent: resp.Message.ReasoningContent})
				a.push(llm.Message{Role: "user", Content: llm.Str(fmt.Sprintf(
					"(harness) You have reasoned for ~%d tokens without acting; that is enough to act on. Do not draft code in your reasoning: make the tool call now (write or edit the files directly, then run the tests). You can refine after seeing the results.", a.cfg.ThinkBudgetTokens))})
				a.noThinkNext = true
				continue
			}
			a.push(llm.Message{Role: "user", Content: llm.Str(fmt.Sprintf(
				"(harness) Your reasoning for this step exceeded ~%d tokens without acting. Settle on the simplest approach that meets the requirements, then make the tool call. You can refine it after running the tests.", a.cfg.ThinkBudgetTokens))})
			continue
		}
		a.overBudget = false
		if err != nil {
			if errors.Is(err, llm.ErrContextOverflow) && !a.IsSub {
				a.code(trace.F5Context)
				a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "context_overflow", Code: trace.F5Context, Detail: err.Error()})
				if cerr := a.compact(ctx, turn, "overflow"); cerr == nil {
					continue
				}
			}
			a.out.Status, a.out.Err = "error", err.Error()
			a.code(trace.F8Infra)
			a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "llm_error", Code: trace.F8Infra, Detail: err.Error()})
			return a.finalize()
		}
		a.account(turn, effort, resp)
		msg := resp.Message

		// Repair: tool call markup leaked into content.
		if len(msg.ToolCalls) == 0 && repair.LooksLikeToolCall(msg.Text()) {
			if calls := repair.ParseContent(msg.Text()); len(calls) > 0 {
				msg.ToolCalls = calls
				txt := msg.Text()
				if i := strings.Index(txt, "<tool_call>"); i >= 0 {
					txt = txt[:i]
				} else if i := strings.Index(txt, "<function="); i >= 0 {
					txt = txt[:i]
				}
				msg.Content = llm.Str(strings.TrimSpace(txt))
				a.out.Repaired++
				a.code(trace.F1ToolFormat)
				a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "repaired_tool_call", Code: trace.F1ToolFormat})
			}
		}
		a.push(msg)

		if len(msg.ToolCalls) == 0 {
			a.lastTurnBad = true
			a.nudges++
			a.out.Nudges++
			if a.nudges > a.cfg.MaxNudges {
				a.out.Status = "gave_up"
				a.out.Summary = msg.Text()
				a.code(trace.F10GaveUp)
				return a.finalize()
			}
			var nudge string
			if resp.FinishReason == "length" {
				a.code(trace.F4ThinkOverflow)
				a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "think_overflow", Code: trace.F4ThinkOverflow})
				nudge = "Your previous response hit the output token limit before you acted. Think less: decide on ONE concrete next step and make the tool call now."
			} else {
				a.code(trace.F1ToolFormat)
				a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "no_tool_call", Code: trace.F1ToolFormat, Detail: trunc(msg.Text(), 300)})
				nudge = "You replied without calling a tool. Describing a call in text does nothing. Make an actual tool call now. If the task is complete and verified, call the finish tool."
			}
			a.push(llm.Message{Role: "user", Content: llm.Str(nudge)})
			continue
		}
		a.nudges = 0

		done := false
		errsBefore := a.out.ToolErrors
		for i, tc := range msg.ToolCalls {
			res, isDone := a.handleCall(ctx, turn, i, tc)
			a.push(llm.Message{Role: "tool", ToolCallID: tc.ID, Name: tc.Function.Name, Content: llm.Str(res)})
			if isDone {
				done = true
			}
		}
		a.lastTurnBad = a.out.ToolErrors > errsBefore
		if done {
			return a.finalize()
		}
	}
}

// effort returns the reasoning effort for a turn. The effort is rendered into
// the prompt prefix, so any change between turns forces a full re-prefill;
// PlannerEffort != ReasoningEffort therefore costs one re-prefill on turn 2.
func (a *Agent) effort(turn int) string {
	if turn == 1 && a.cfg.PlannerEffort != "" {
		return a.cfg.PlannerEffort
	}
	return a.cfg.ReasoningEffort
}

// thinkThisTurn applies ThinkingMode. "adaptive" thinks on the first two
// turns and whenever the previous turn ended badly (tool error, failing
// test/check, no tool call); routine turns after successful tools don't.
func (a *Agent) thinkThisTurn(turn int) bool {
	switch a.cfg.ThinkingMode {
	case "off":
		return false
	case "adaptive":
		return turn <= 2 || a.lastTurnBad
	}
	return true
}

// warmPrefix sends a 1-token request whose prompt is just the stable prefix
// (system + tools + a trivial user turn). The server checkpoints prompt state
// at the end of each request, so this leaves a checkpoint right after the
// shared prefix that every task, sub-agent and post-compaction segment can
// reuse (docs/ENDPOINT.md). When the prefix is already cached it costs ~2 s.
func (a *Agent) warmPrefix(ctx context.Context) {
	if a.cfg.NoWarmup {
		return
	}
	msgs := []llm.Message{a.msgs[0], {Role: "user", Content: llm.Str(".")}}
	r, err := a.Client.Chat(ctx, llm.Request{Messages: msgs, Tools: a.Tools, MaxTokens: 1, ReasoningEffort: a.effort(1)})
	if err != nil {
		a.logf("prefix warm-up failed: %v", err)
		return
	}
	a.Tr.Event(trace.Event{Agent: a.Name, Name: "warmup", Data: map[string]any{"prompt_tokens": r.Usage.PromptTokens, "cached_tokens": r.Usage.CachedTokens, "latency_ms": r.LatencyMS}})
}

func (a *Agent) account(turn int, effort string, r *llm.Response) {
	u := r.Usage
	// The server caches the previous prompt but not generated tokens, so the
	// expected cache hit is the previous prompt (minus page alignment).
	reprefill := 0
	if a.prevProm > 0 && u.CachedTokens < a.prevProm-512 {
		reprefill = a.prevProm - u.CachedTokens
	}
	a.out.PromptTokens += u.PromptTokens
	a.out.PrefillTokens += u.PrefillTokens
	a.out.ReprefillTokens += reprefill
	a.out.CompletionTokens += u.CompletionTokens
	a.out.ReasoningTokens += u.ReasoningTokens
	a.Tr.Turn(trace.Turn{Agent: a.Name, Segment: a.segment, Turn: turn, PromptTokens: u.PromptTokens, CachedTokens: u.CachedTokens,
		PrefillTokens: u.PrefillTokens, ReprefillTokens: reprefill, CompletionTokens: u.CompletionTokens, ReasoningTokens: u.ReasoningTokens,
		LatencyMS: r.LatencyMS, TTFTMS: r.TTFTMS, FinishReason: r.FinishReason, CacheStatus: r.CacheStatus, ToolCalls: len(r.Message.ToolCalls),
		Retries: r.Retries, Effort: effort})
	a.logf("turn %d: prompt=%d cached=%d prefill=%d out=%d (think %d) %.1fs finish=%s tools=%d%s",
		turn, u.PromptTokens, u.CachedTokens, u.PrefillTokens, u.CompletionTokens, u.ReasoningTokens, float64(r.LatencyMS)/1000, r.FinishReason, len(r.Message.ToolCalls),
		map[bool]string{true: fmt.Sprintf(" REPREFILL=%d", reprefill), false: ""}[reprefill > 0])
	a.lastProm = u.PromptTokens + u.CompletionTokens
	a.prevProm = u.PromptTokens
}

// handleCall executes one tool call and returns the tool message content.
func (a *Agent) handleCall(ctx context.Context, turn, idx int, tc llm.ToolCall) (string, bool) {
	t0 := time.Now()
	name := tc.Function.Name
	rawArgs := tc.Function.Arguments
	a.out.ToolCalls++
	args, err := tools.ParseArgs(rawArgs)
	if err != nil {
		if fixed, ok := repair.RepairArgs(rawArgs); ok {
			args, err = tools.ParseArgs(fixed)
			a.out.Repaired++
		}
	}
	var res tools.Result
	finished := false
	switch {
	case a.allowed != nil && !a.allowed[name]:
		res = tools.Result{ErrorClass: "not_allowed", Output: fmt.Sprintf("ERROR not_allowed: you are a read-only sub-agent; %s is not available. Use read_file, grep, list_dir or run (read-only commands), then call finish with your report.", name)}
		a.code(trace.F2WrongTool)
	case err != nil:
		res = tools.Result{ErrorClass: "bad_args", Output: fmt.Sprintf("ERROR bad_args: arguments are not valid JSON (%v). Re-issue the call with valid arguments.", err)}
		a.code(trace.F1ToolFormat)
	case name == "finish":
		res, finished = a.finish(ctx, turn, args)
	case name == "update_plan":
		res = a.updatePlan(args)
	case name == "delegate":
		res = a.delegate(ctx, args)
	default:
		res = a.Env.Exec(ctx, name, args)
		if res.ErrorClass == "unknown_tool" {
			a.code(trace.F2WrongTool)
		}
		if a.cfg.VerifyWrites && res.OK && (name == "write_file" || name == "edit_file") {
			res.Output += a.verifyWrite(ctx, res.Changed)
		}
	}
	if !res.OK && res.ErrorClass != "" {
		a.out.ToolErrors++
	}
	out := res.Output
	// Error-signature hints (Harness² plugin pattern).
	for i := range a.cfg.ErrorHints {
		h := &a.cfg.ErrorHints[i]
		if h.Match(name, out) {
			key := fmt.Sprintf("hint%d", i)
			a.errSeen[key]++
			if a.errSeen[key] <= 3 {
				out += "\nHINT: " + h.Text
			}
		}
	}
	spillName := fmt.Sprintf("%s-t%d-%d-%s", a.Name, turn, idx, name)
	capped, spilled := ctxmgr.Cap(a.Env, spillName, out, a.cfg.ToolCapTokens)
	progress := len(res.Changed) > 0 || res.TestSignal != ""
	if hint, cls := a.loops.Observe(name, rawArgs, progress); hint != "" && !finished {
		capped += "\n\n" + hint
		a.out.LoopHints++
		a.code(trace.F3Loop)
		a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "loop_" + cls, Code: trace.F3Loop, Detail: name})
	}
	a.Tr.Tool(trace.Tool{Agent: a.Name, Turn: turn, Tool: name, ArgsHash: repair.Hash(name, rawArgs), ArgsPreview: trunc(rawArgs, 200),
		OK: res.OK, ErrorClass: res.ErrorClass, OutChars: len(out), Spilled: spilled, Changed: res.Changed, TestSignal: res.TestSignal,
		DurMS: time.Since(t0).Milliseconds()})
	a.logf("  %s %s -> %s", name, trunc(oneLine(rawArgs), 120), trunc(oneLine(firstLine(capped)), 120))
	return capped, finished
}

// verifyWrite is a "plugin" hook: check a write immediately after it happens.
func (a *Agent) verifyWrite(ctx context.Context, changed []string) string {
	var notes []string
	for _, f := range changed {
		if strings.HasSuffix(f, ".go") {
			out, code := a.Env.RunCmd(ctx, "command -v gofmt >/dev/null || exit 0; gofmt -l -e "+shellQuote(f)+" 2>&1 >/dev/null | head -8", 20*time.Second)
			if code == 0 && strings.TrimSpace(out) != "" {
				notes = append(notes, "WARNING syntax errors in "+f+" (gofmt):\n"+strings.TrimSpace(out))
			}
		}
		if strings.HasSuffix(f, ".json") {
			out, code := a.Env.RunCmd(ctx, "command -v python3 >/dev/null || exit 0; python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "+shellQuote(f)+" 2>&1 | tail -1", 20*time.Second)
			if code == 0 && strings.TrimSpace(out) != "" {
				notes = append(notes, "WARNING invalid JSON in "+f+": "+strings.TrimSpace(out))
			}
		}
	}
	if len(notes) == 0 {
		return ""
	}
	return "\n" + strings.Join(notes, "\n")
}

func (a *Agent) finish(ctx context.Context, turn int, args tools.Args) (tools.Result, bool) {
	summary, _ := args.Str("summary")
	a.out.Summary = summary
	if a.cfg.ReviewBeforeFinish && !a.IsSub && !a.reviewed {
		a.reviewed = true
		a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "finish_review"})
		return tools.Result{OK: false, ErrorClass: "review_required", Output: "REVIEW REQUIRED before finishing (one time). Re-read the task statement above. " +
			"Write a checklist of EVERY explicit requirement in it (signatures, edge cases, error conditions, output formats, files to update). " +
			"For each item, say where it is implemented (file:line) and which test covers it. If any item is missing or untested, fix it and run the tests. " +
			"Then call finish again."}, false
	}
	a.out.FinishAttempts++
	if a.IsSub || a.Task.Check == "" || !a.cfg.CheckOnFinish {
		a.out.Status = "finished"
		return tools.Result{OK: true, Output: "OK finished"}, true
	}
	out, code := a.Env.RunCmd(ctx, a.Task.Check, 10*time.Minute)
	timedOut := false
	a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "acceptance_check", Detail: fmt.Sprintf("exit=%d timeout=%v", code, timedOut)})
	if code == 0 && !timedOut {
		a.out.Status = "finished"
		a.out.CheckPassed = true
		return tools.Result{OK: true, Output: "OK acceptance check passed. Task complete."}, true
	}
	a.code(trace.F6ClaimedDone)
	if a.out.FinishAttempts >= a.cfg.MaxFinishAttempts {
		a.out.Status = "check_failed"
		return tools.Result{OK: false, ErrorClass: "check_failed", Output: "ERROR check_failed: giving up after repeated failures"}, true
	}
	return tools.Result{OK: false, ErrorClass: "check_failed", Output: fmt.Sprintf(
		"ERROR check_failed: the acceptance check `%s` failed (exit=%d), so the task is NOT complete. Read the output, fix the cause, re-run the check yourself, then call finish again.\n%s",
		a.Task.Check, code, out)}, false
}

func (a *Agent) updatePlan(args tools.Args) tools.Result {
	step, _ := args.Str("step")
	status, _ := args.Str("status")
	note, _ := args.Str("note")
	if step == "" {
		return tools.Result{ErrorClass: "missing_arg", Output: "ERROR missing_arg: update_plan needs step"}
	}
	path := filepath.Join(ctxmgr.QHDir(a.Env), "PLAN.md")
	var data []byte
	if a.Env.Container == "" {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		data, _ = os.ReadFile(path)
	} else {
		out, _ := a.Env.RunCmd(context.Background(), "cat "+shellQuote(path)+" 2>/dev/null", 10*time.Second)
		data = []byte(out)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	entry := fmt.Sprintf("- [%s] %s", status, step)
	if note != "" {
		entry += " — " + note
	}
	replaced := false
	for i, l := range lines {
		if planKey(l) == planKey(entry) {
			lines[i] = entry
			replaced = true
		}
	}
	if !replaced {
		lines = append(lines, entry)
	}
	content := strings.Join(lines, "\n") + "\n"
	_ = a.Env.WriteFile(path, []byte(content))
	return tools.Result{OK: true, Output: "OK plan updated:\n" + content}
}

// planKey normalises a plan line to its step name.
func planKey(l string) string {
	if i := strings.Index(l, "] "); i >= 0 {
		l = l[i+2:]
	}
	if i := strings.Index(l, " — "); i >= 0 {
		l = l[:i]
	}
	return strings.ToLower(strings.TrimSpace(l))
}

func (a *Agent) delegate(ctx context.Context, args tools.Args) tools.Result {
	task, _ := args.Str("task")
	if strings.TrimSpace(task) == "" {
		return tools.Result{ErrorClass: "missing_arg", Output: "ERROR missing_arg: delegate needs task"}
	}
	if a.IsSub {
		return tools.Result{ErrorClass: "not_allowed", Output: "ERROR not_allowed: sub-agents cannot delegate"}
	}
	a.subN++
	a.out.Delegations++
	sub := &Agent{
		Name: fmt.Sprintf("%s.sub%d", a.Name, a.subN), Client: a.Client, H: a.H, Env: a.Env, Tr: a.Tr,
		Task: Task{ID: a.Task.ID, Prompt: task}, Log: a.Log, IsSub: true,
		// Same tool definitions as the parent so the rendered prefix (and its
		// cache checkpoint) is shared; the read-only subset is enforced in
		// handleCall.
		Tools: a.Tools, MaxTurn: a.H.Config.SubAgentMaxTurns,
		allowed: toSet(a.H.Config.SubAgentTools),
	}
	o := sub.Run(ctx)
	a.out.PromptTokens += o.PromptTokens
	a.out.PrefillTokens += o.PrefillTokens
	a.out.CompletionTokens += o.CompletionTokens
	a.out.ReasoningTokens += o.ReasoningTokens
	report := strings.TrimSpace(o.Summary)
	if report == "" {
		report = "(sub-agent returned no report; status " + o.Status + ")"
	}
	capped, _ := ctxmgr.Cap(a.Env, sub.Name+"-report", report, 1200)
	return tools.Result{OK: o.Status == "finished", ErrorClass: map[bool]string{true: "", false: "subagent_" + o.Status}[o.Status == "finished"],
		Output: fmt.Sprintf("OK sub-agent report (%d turns, status %s):\n%s", o.Turns, o.Status, capped)}
}

// compact ends the current context segment: the model writes a checkpoint
// (appended, so it is cheap), the harness adds deterministic state, and a new
// segment starts from the stable prefix + task card + checkpoint.
func (a *Agent) compact(ctx context.Context, turn int, reason string) error {
	a.logf("compacting (%s) at %d prompt tokens", reason, a.lastProm)
	req := "CONTEXT CHECKPOINT. The conversation is about to be reset to save space; only what you write now survives (plus the task statement and a git status). " +
		"Do NOT call tools. Write a checkpoint in markdown with these sections:\n" +
		"## Progress\nWhat is done and verified (files, functions).\n## Key facts\nFacts you discovered that you would otherwise have to rediscover (paths, APIs, commands, error causes).\n" +
		"## Decisions\nDecisions made and why.\n## Open issues\nWhat is failing or unknown.\n## Next steps\nThe exact next 1-3 actions.\nBe concrete and brief (under 600 words)."
	var summary string
	if reason == "threshold" {
		msgs := append(append([]llm.Message{}, a.msgs...), llm.Message{Role: "user", Content: llm.Str(req)})
		// Same tools and effort as the segment so the request is a cache hit
		// on the whole conversation (only the checkpoint request is new).
		resp, err := a.Client.Chat(ctx, llm.Request{Messages: msgs, Tools: a.Tools, MaxTokens: a.cfg.CompactSummaryMax + 2000, ReasoningEffort: a.effort(turn)})
		if err == nil {
			a.account(turn, a.effort(turn), resp)
			summary = strings.TrimSpace(resp.Message.Text())
			if summary == "" {
				// The model answered with a tool call; its reasoning is the best
				// available record of the state.
				summary = strings.TrimSpace(resp.Message.ReasoningContent)
			}
		} else {
			a.logf("checkpoint summary failed: %v", err)
		}
	}
	if summary == "" {
		summary = "(no model-written checkpoint available)"
	}
	state := ctxmgr.StateSnapshot(ctx, a.Env)
	a.segment++
	a.out.Compactions++
	cp := fmt.Sprintf("# Checkpoint %d (context was reset after %d turns)\nYou already worked on this task. Below is your own checkpoint and the current workspace state. Continue from \"Next steps\". Re-read files before editing them; your earlier views of them are gone.\n\n%s\n\n## Workspace state (generated)\n%s",
		a.segment, turn-1, summary, state)
	dir := filepath.Join(ctxmgr.QHDir(a.Env), "state")
	if a.Env.Container == "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	_ = a.Env.WriteFile(filepath.Join(dir, fmt.Sprintf("CHECKPOINT-%d.md", a.segment)), []byte(cp))
	a.Tr.Event(trace.Event{Agent: a.Name, Turn: turn, Name: "compaction", Detail: reason, Data: map[string]any{"prompt_tokens": a.lastProm, "checkpoint_chars": len(cp)}})
	a.msgs = nil
	a.push(llm.Message{Role: "system", Content: llm.Str(a.H.System)})
	a.push(llm.Message{Role: "user", Content: llm.Str(a.TaskCard() + "\n\n" + cp)})
	a.lastProm = 0
	a.prevProm = 0
	a.loops = repair.NewLoopDetector()
	return nil
}

func (a *Agent) finalize() Outcome {
	if a.out.Status == "" {
		a.out.Status = "finished"
	}
	a.out.WallSeconds = time.Since(a.start).Seconds()
	a.Tr.Event(trace.Event{Agent: a.Name, Turn: a.out.Turns, Name: "end", Detail: a.out.Status, Data: map[string]any{"check_passed": a.out.CheckPassed}})
	return a.out
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func toSet(names []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}
