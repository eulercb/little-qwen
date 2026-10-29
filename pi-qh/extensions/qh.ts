// qh it2 on pi: the mechanisms that took qh from 0.654 to 0.903 on Tier B,
// ported to pi extension events. Load with:
//   pi -e pi-qh/extensions/qh.ts ...
// Configuration: see src/config.ts (defaults = harness/it2).
//
// Mechanism map (qh → pi):
//   L1 think cap (8K, keep reasoning, act turn with thinking off)
//       message_update(thinking_delta) → ctx.abort(); message_end keeps the
//       aborted message's reasoning (pi drops aborted messages otherwise);
//       agent_before_settle adds the act nudge and continues; the next request
//       gets chat_template_kwargs.enable_thinking=false in before_provider_request.
//   check on finish            agent_before_settle runs $QH_CHECK; failure → feedback + continue
//   output-limit recovery      agent_before_settle on stopReason "length" → nudge + continue
//   budget (80 turns / 60 min) turn_start → ctx.abort(); settle restores the last green tree (L3)
//   L2 test-after-write        tool_result(write) note after 300 untested lines
//   L3 restore green           tool_result(bash test pass) snapshots the tree via a throwaway git index
//   fuzzy edit                 tool_call(edit) rewrites oldText/newText from a whitespace-tolerant match
//   verify writes              tool_result(write/edit) gofmt / JSON check
//   loop detector              tool_result hint on repeats / no progress
//   output cap + spill         tool_result over 4K tokens → head+tail, full text in .qh/spill
//   read window, bash timeout  tool_call clamps read.limit (400) and bash.timeout (120 s, max 600)
//   update_plan tool, qh system prompt, qh task card
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, isAbsolute, join, relative } from "node:path";
import { CHARS_PER_TOKEN, loadConfig } from "../src/config.ts";
import { countOf, fuzzyFix } from "../src/fuzzy.ts";
import { systemPrompt, taskCard } from "../src/prompt.ts";
import { Trace, canonical } from "../src/trace.ts";
import { registerChunks, type Chunks } from "../src/chunks.ts";

// qh's testCmdRe (internal/tools/tools.go).
const TEST_CMD = /\b(go (test|build|vet)|npm (run )?(test|build)|pytest|make|cargo (test|build))\b/;

// qh's snapshotCmd: the workdir (tracked + untracked, honoring .gitignore) as a
// git tree through a throwaway index, so the real index is untouched.
const SNAPSHOT = `d=$(mktemp -d) && GIT_INDEX_FILE=$d/i git add -A . >/dev/null 2>&1 && GIT_INDEX_FILE=$d/i git write-tree; rm -rf "$d"`;

const ACT_PREFIX = "(harness) You have reasoned for ~";
const ACT_NUDGE = (n: number) =>
  `(harness) You have reasoned for ~${n} tokens without acting; that is enough to act on. Do not draft code in your reasoning: make the tool call now (write or edit the files directly, then run the tests). You can refine after seeing the results.`;
const LENGTH_NUDGE =
  "Your previous response hit the output token limit before you acted. Think less: decide on ONE concrete next step and make the tool call now.";
const EMPTY_NUDGE =
  "You replied without calling a tool and without a summary. Make an actual tool call now. If the task is complete and verified, reply with a short summary.";

type Text = { type: "text"; text: string };

export default function (pi: ExtensionAPI) {
  const cfg = loadConfig();
  const check = process.env.QH_CHECK?.trim() || undefined;
  const trace = new Trace(process.env.QH_PI_TRACE, process.env.QH_PI_STATUS);
  const chunks: Chunks | undefined = cfg.chunks ? registerChunks(pi, cfg, trace) : undefined;

  let s = fresh();
  function fresh() {
    return {
      start: Date.now(),
      turns: 0,
      stop: "" as "" | "max_turns" | "wall_clock",
      taskCarded: false,
      // L1
      thinkChars: 0,
      sawContent: false,
      capped: false,
      overBudget: false,
      noThinkNext: false,
      thinkCaps: 0,
      cuts: [] as string[], // reasoning of capped turns, re-inserted before each act nudge
      // settle
      nudges: 0,
      finishAttempts: 0,
      checkPassed: false,
      status: "",
      last: { stopReason: "", toolCalls: 0, text: "" },
      // L2
      untested: 0,
      testNudged: false,
      // L3
      greenTree: "",
      greenCmd: "",
      dirty: false,
      restored: false,
      // loops
      recent: [] as string[],
      noProgress: 0,
      loopHints: 0,
      compactions: 0,
      fuzzyNotes: new Map<string, string[]>(),
    };
  }

  const sh = async (cwd: string, cmd: string, timeoutMs: number) => {
    const r = await pi.exec("bash", ["-c", cmd], { cwd, timeout: timeoutMs });
    return { out: (r.stdout ?? "") + (r.stderr ?? ""), code: r.killed ? 124 : r.code };
  };

  const finalStatus = () => {
    trace.status({
      status: s.status || "finished",
      check_passed: s.checkPassed,
      turns: s.turns,
      finish_attempts: s.finishAttempts,
      think_caps: s.thinkCaps,
      nudges: s.nudges,
      loop_hints: s.loopHints,
      restored_green: s.restored,
      compactions: s.compactions,
      wall_seconds: (Date.now() - s.start) / 1000,
    });
  };

  pi.on("session_start", async () => {
    s = fresh();
    trace.event("start", { config: cfg, check: check ?? null });
  });

  pi.on("session_compact", async (event) => {
    s.compactions++;
    trace.event("compaction", { turn: s.turns, reason: (event as any).reason });
  });

  // ── prompt and task card ────────────────────────────────────────────────
  pi.on("before_agent_start", async () => {
    if (cfg.qhPrompt) return { systemPrompt: systemPrompt({ planTool: cfg.planTool }) };
  });

  pi.on("input", async (event, ctx) => {
    if (s.taskCarded || event.source === "extension" || (!cfg.taskCard && !chunks)) return { action: "continue" as const };
    s.taskCarded = true;
    const card = cfg.taskCard ? taskCard(event.text, ctx.cwd, check) : event.text;
    chunks?.setTaskCard(card);
    return { action: "transform" as const, text: card + (chunks?.planInstruction() ?? ""), images: event.images };
  });

  // ── budget ─────────────────────────────────────────────────────────────
  pi.on("turn_start", async (_event, ctx) => {
    s.turns++;
    s.thinkChars = 0;
    s.sawContent = false;
    if (cfg.maxTurns > 0 && s.turns > cfg.maxTurns) s.stop = "max_turns";
    else if (cfg.maxWallMinutes > 0 && Date.now() - s.start > cfg.maxWallMinutes * 60_000) s.stop = "wall_clock";
    if (s.stop) {
      trace.event("budget_stop", { reason: s.stop, turn: s.turns });
      ctx.abort();
    }
  });

  // ── L1: think cap ──────────────────────────────────────────────────────
  pi.on("message_update", async (event, ctx) => {
    const ev: any = event.assistantMessageEvent;
    if (!ev) return;
    if (ev.type === "text_delta" || ev.type === "toolcall_start") s.sawContent = true;
    if (ev.type !== "thinking_delta") return;
    s.thinkChars += typeof ev.delta === "string" ? ev.delta.length : 0;
    if (cfg.thinkBudgetTokens <= 0 || s.overBudget || s.capped || s.sawContent) return;
    if (s.thinkChars > cfg.thinkBudgetTokens * 3.5) {
      s.capped = true;
      s.thinkCaps++;
      trace.event("think_budget", { turn: s.turns, reasoning_chars: s.thinkChars });
      ctx.abort();
    }
  });

  pi.on("before_provider_request", async (event) => {
    if (!s.noThinkNext && s.cuts.length === 0) return;
    const p: any = { ...(event.payload as any) };
    if (s.cuts.length > 0 && Array.isArray(p.messages)) {
      // pi never replays a reasoning-only assistant message (no text, no tool
      // calls), so put each capped turn back exactly as qh sends it: empty
      // content + reasoning_content, right before its act nudge. Done on every
      // request so the history (and the cached prefix) stays stable.
      const out: any[] = [];
      let k = 0;
      for (const m of p.messages) {
        const body = typeof m.content === "string" ? m.content : Array.isArray(m.content) ? m.content.map((c: any) => c.text ?? "").join("") : "";
        if (m.role === "user" && body.startsWith(ACT_PREFIX) && k < s.cuts.length) {
          out.push({ role: "assistant", content: "", reasoning_content: s.cuts[k++] });
        }
        out.push(m);
      }
      p.messages = out;
    }
    if (s.noThinkNext) {
      // Thinking on/off only changes the generation-prompt suffix on this server,
      // so it keeps the cached prefix (docs/ENDPOINT.md); Qwen's non-thinking sampling.
      p.chat_template_kwargs = { ...(p.chat_template_kwargs ?? {}), enable_thinking: false };
      delete p.reasoning_effort;
      p.temperature = 0.7;
      p.top_p = 0.8;
      p.presence_penalty = 1.5;
    }
    return p;
  });

  pi.on("message_end", async (event) => {
    const m: any = event.message;
    if (m?.role !== "assistant") return;
    const content: any[] = Array.isArray(m.content) ? m.content : [];
    const toolCalls = content.filter((b) => b.type === "toolCall").length;
    const text = content.filter((b) => b.type === "text").map((b) => b.text).join("");
    const reasoning = content.filter((b) => b.type === "thinking").map((b) => b.thinking ?? "").join("");
    trace.turn({
      turn: s.turns,
      prompt_tokens: (m.usage?.input ?? 0) + (m.usage?.cacheRead ?? 0),
      cached_tokens: m.usage?.cacheRead ?? 0,
      completion_tokens: m.usage?.output ?? 0,
      reasoning_chars: reasoning.length,
      stop_reason: m.stopReason,
      tool_calls: toolCalls,
      no_think: s.noThinkNext,
    });
    if (s.noThinkNext && m.stopReason !== "aborted") {
      s.noThinkNext = false;
    }
    if (!s.capped) {
      s.overBudget = false;
      s.last = { stopReason: m.stopReason, toolCalls, text };
      if (toolCalls > 0) s.nudges = 0;
      return;
    }
    // Capped turn: pi drops aborted assistant messages from the next request
    // (pi-ai transform-messages), which is EXP-3's rejected "discard" variant.
    // think_cap_continue keeps the partial reasoning (re-inserted in
    // before_provider_request).
    s.last = { stopReason: "think_cap", toolCalls: 0, text: "" };
    if (cfg.thinkCapContinue && reasoning) s.cuts.push(reasoning);
  });

  // ── settle: act nudge, output-limit recovery, check on finish, budget ──────
  pi.on("agent_before_settle", async (event, ctx) => {
    const cont = (text: string, name: string) => {
      trace.event(name, { turn: s.turns });
      return { entries: [{ type: "custom_message" as const, customType: "qh", content: text, display: true }], continue: true };
    };
    // Aborted runs (think cap, budget) never reach this boundary: pi skips it
    // after ctx.abort(); they are handled in agent_settled below.
    if (event.outcome === "error") {
      s.status = "error";
      finalStatus();
      return;
    }
    if (event.outcome === "aborted") {
      s.status = s.status || "aborted";
      finalStatus();
      return;
    }
    if (s.last.toolCalls > 0) return; // pi continues by itself
    if (s.last.stopReason === "length" || (!s.last.text.trim() && s.last.stopReason !== "think_cap")) {
      s.nudges++;
      if (s.nudges > cfg.maxNudges) {
        s.status = "gave_up";
        finalStatus();
        return;
      }
      return s.last.stopReason === "length" ? cont(LENGTH_NUDGE, "think_overflow") : cont(EMPTY_NUDGE, "no_tool_call");
    }
    const pending = chunks?.pendingNudge();
    if (pending) {
      s.nudges++;
      if (s.nudges > cfg.maxNudges) {
        s.status = "gave_up";
        finalStatus();
        return;
      }
      return cont(pending, "chunk_nudge");
    }
    // A text reply ends the task in pi: that is qh's finish call.
    s.finishAttempts++;
    if (!cfg.checkOnFinish || !check) {
      s.status = "finished";
      finalStatus();
      return;
    }
    const r = await sh(ctx.cwd, check, 10 * 60_000);
    trace.event("acceptance_check", { exit: r.code, attempt: s.finishAttempts });
    if (r.code === 0) {
      s.status = "finished";
      s.checkPassed = true;
      finalStatus();
      return;
    }
    if (s.finishAttempts >= cfg.maxFinishAttempts) {
      s.status = "check_failed";
      finalStatus();
      return;
    }
    return cont(
      `ERROR check_failed: the acceptance check \`${check}\` failed (exit=${r.code}), so the task is NOT complete. Read the output, fix the cause, re-run the check yourself, then finish again.\n` +
        capText(ctx.cwd, `check-${s.finishAttempts}`, r.out),
      "check_failed",
    );
  });

  // After ctx.abort() pi skips agent_before_settle, so the think-cap act turn
  // starts a new run from here (pi defers it until settling finishes; print
  // mode waits for it). The capped message's reasoning is already in the
  // transcript (message_end above).
  pi.on("agent_settled", async (_event, ctx) => {
    if (s.capped && !s.stop) {
      s.capped = false;
      s.overBudget = true;
      s.noThinkNext = true;
      trace.event("act_nudge", { turn: s.turns });
      pi.sendMessage({ customType: "qh", content: ACT_NUDGE(cfg.thinkBudgetTokens), display: true }, { triggerTurn: true });
      return;
    }
    if (s.stop && !s.status) {
      s.status = s.stop;
      await restoreGreen(ctx.cwd);
    }
    finalStatus();
  });

  // ── tool calls: read window, bash timeout, fuzzy edit ─────────────────────
  pi.on("tool_call", async (event, ctx) => {
    const input: any = (event as any).input ?? {};
    switch (event.toolName) {
      case "bash": {
        const t = typeof input.timeout === "number" && input.timeout > 0 ? input.timeout : cfg.bashDefaultTimeout;
        if (cfg.bashDefaultTimeout > 0) input.timeout = Math.min(t, cfg.bashMaxTimeout);
        return;
      }
      case "read":
        if (cfg.readMaxLines > 0 && (typeof input.limit !== "number" || input.limit > cfg.readMaxLines)) {
          input.limit = cfg.readMaxLines;
        }
        return;
      case "edit": {
        if (!cfg.fuzzyEdit || typeof input.path !== "string" || !Array.isArray(input.edits)) return;
        let file: string;
        try {
          file = readFileSync(isAbsolute(input.path) ? input.path : join(ctx.cwd, input.path), "utf8");
        } catch {
          return;
        }
        const notes: string[] = [];
        for (const e of input.edits) {
          if (typeof e?.oldText !== "string" || typeof e?.newText !== "string") continue;
          if (countOf(file, e.oldText) > 0) continue;
          const fix = fuzzyFix(file, e.oldText, e.newText);
          if (!fix) continue;
          e.oldText = fix.oldText;
          e.newText = fix.newText;
          notes.push(`edit at line ${fix.line}: ${fix.note}`);
        }
        if (notes.length) {
          s.fuzzyNotes.set(event.toolCallId, notes);
          trace.event("fuzzy_edit", { turn: s.turns, notes });
        }
        return;
      }
    }
  });

  // ── tool results: verify, L2, L3, loops, cap ────────────────────────────
  pi.on("tool_result", async (event, ctx) => {
    const name = (event as any).toolName as string;
    const input: any = event.input ?? {};
    let extra = "";
    let progress = false;
    let testSignal = "";
    if ((name === "write" || name === "edit") && !event.isError) {
      progress = true;
      s.dirty = true;
      const notes = s.fuzzyNotes.get(event.toolCallId);
      if (notes) extra += "\nNOTE: " + notes.join("; ");
      if (cfg.verifyWrites && typeof input.path === "string") extra += await verifyWrite(ctx.cwd, input.path);
    }
    if (name === "write" && !event.isError && cfg.testAfterWriteLines > 0) {
      const content = typeof input.content === "string" ? input.content : "";
      s.untested += countOf(content, "\n") + 1;
      if (s.untested >= cfg.testAfterWriteLines && !s.testNudged) {
        s.testNudged = true;
        trace.event("test_nudge", { turn: s.turns, lines: s.untested });
        extra += `\nNOTE (harness): about ${s.untested} lines written since the last build/test run. Build and run the tests now, before reasoning further; then fix failures with edit instead of rewriting whole files.`;
      }
    }
    if (name === "bash" && typeof input.command === "string" && TEST_CMD.test(input.command)) {
      testSignal = event.isError ? "fail" : "pass";
      progress = true;
      if (cfg.testAfterWriteLines > 0) {
        s.untested = 0;
        s.testNudged = false;
      }
      if (testSignal === "pass" && cfg.restoreGreen) {
        const r = await sh(ctx.cwd, SNAPSHOT, 30_000);
        const tree = r.out.trim();
        if (r.code === 0 && /^[0-9a-f]{40}$/.test(tree)) {
          s.greenTree = tree;
          s.greenCmd = input.command;
          s.dirty = false;
        }
      }
    }
    if (cfg.loopRepeat > 0 || cfg.loopNoProgress > 0) {
      const hint = observeLoop(name, input, progress);
      if (hint) {
        extra += "\n\n" + hint;
        s.loopHints++;
      }
    }
    const texts = (event.content ?? []).filter((c: any) => c.type === "text") as Text[];
    const body = texts.map((t) => t.text).join("\n");
    const capped = cfg.toolCapTokens > 0 ? capText(ctx.cwd, `t${s.turns}-${name}`, body) : body;
    if (capped === body && !extra) return;
    const others = (event.content ?? []).filter((c: any) => c.type !== "text");
    return { content: [...others, { type: "text", text: capped + extra }] as any };
  });

  // ── update_plan (qh's plan tool; PLAN.md under .qh/) ─────────────────────
  if (cfg.planTool) {
    pi.registerTool({
      name: "update_plan",
      label: "update_plan",
      description:
        "Record or update one step of your plan (kept in .qh/PLAN.md). Call once per step; calling again with the same step name updates its status.",
      parameters: Type.Object({
        step: Type.String({ description: "Short step name" }),
        status: Type.String({ description: "todo | doing | done | blocked" }),
        note: Type.Optional(Type.String({ description: "Optional one-line note" })),
      }),
      async execute(_id: string, params: any, _signal: any, _onUpdate: any, ctx: any) {
        const out = updatePlan(ctx.cwd, params);
        return { content: [{ type: "text", text: out }], details: undefined };
      },
    } as any);
  }

  // ── helpers ──────────────────────────────────────────────────────────────
  function observeLoop(name: string, input: unknown, progress: boolean): string {
    const h = createHash("sha1").update(name + "\0" + canonical(input)).digest("hex").slice(0, 16);
    s.recent.push(h);
    if (s.recent.length > cfg.loopWindow) s.recent = s.recent.slice(-cfg.loopWindow);
    s.noProgress = progress ? 0 : s.noProgress + 1;
    const n = s.recent.filter((r) => r === h).length;
    if (cfg.loopRepeat > 0 && n >= cfg.loopRepeat) {
      s.recent = [];
      trace.event("loop_repeat", { turn: s.turns, tool: name });
      return `LOOP WARNING: you have made this exact ${name} call ${n} times recently and got the same result. Stop repeating it. State in one sentence what you learned, then choose a different action.`;
    }
    if (cfg.loopNoProgress > 0 && s.noProgress >= cfg.loopNoProgress) {
      s.noProgress = 0;
      trace.event("loop_no_progress", { turn: s.turns });
      return `PROGRESS WARNING: ${cfg.loopNoProgress} tool calls without changing any file or running a build/test. Stop exploring: write down your hypothesis, make the smallest concrete change, then run the tests.`;
    }
    return "";
  }

  async function verifyWrite(cwd: string, path: string): Promise<string> {
    const rel = isAbsolute(path) ? relative(cwd, path) : path;
    const q = "'" + rel.replace(/'/g, `'\\''`) + "'";
    if (rel.endsWith(".go")) {
      const r = await sh(cwd, `command -v gofmt >/dev/null || exit 0; gofmt -l -e ${q} 2>&1 >/dev/null | head -8`, 20_000);
      if (r.code === 0 && r.out.trim()) return `\nWARNING syntax errors in ${rel} (gofmt):\n${r.out.trim()}`;
    }
    if (rel.endsWith(".json")) {
      const r = await sh(cwd, `command -v python3 >/dev/null || exit 0; python3 -c 'import json,sys; json.load(open(sys.argv[1]))' ${q} 2>&1 | tail -1`, 20_000);
      if (r.code === 0 && r.out.trim()) return `\nWARNING invalid JSON in ${rel}: ${r.out.trim()}`;
    }
    return "";
  }

  // qh's ctxmgr.Cap: head 55% + tail 35% + a pointer to the full text.
  function capText(cwd: string, name: string, out: string): string {
    const limit = Math.floor(cfg.toolCapTokens * CHARS_PER_TOKEN);
    if (cfg.toolCapTokens <= 0 || out.length <= limit) return out;
    const rel = join(".qh", "spill", name.replace(/[^\w.-]/g, "_") + ".txt");
    try {
      mkdirSync(dirname(join(cwd, rel)), { recursive: true });
      writeFileSync(join(cwd, rel), out);
    } catch {}
    const lines = countOf(out, "\n") + 1;
    const headN = Math.floor((limit * 55) / 100);
    const tailN = Math.floor((limit * 35) / 100);
    let head = out.slice(0, headN);
    const hi = head.lastIndexOf("\n");
    if (hi > headN / 2) head = head.slice(0, hi + 1);
    let tail = out.slice(out.length - tailN);
    const ti = tail.indexOf("\n");
    if (ti >= 0 && ti < tailN / 2) tail = tail.slice(ti + 1);
    const omitted = countOf(out.slice(head.length, out.length - tail.length), "\n");
    const est = Math.floor(out.length / CHARS_PER_TOKEN) + 1;
    return `${head}\n... [${omitted} lines omitted: output was ${lines} lines / ~${est} tokens, over the ${cfg.toolCapTokens}-token inline cap. Full output saved to ${rel} - use read with offset/limit or grep on it to see more.] ...\n${tail}`;
  }

  async function restoreGreen(cwd: string): Promise<void> {
    if (!cfg.restoreGreen || !s.greenTree || !s.dirty) return;
    if ((await sh(cwd, s.greenCmd, 3 * 60_000)).code === 0) return;
    const restore = `d=$(mktemp -d) && GIT_INDEX_FILE=$d/i git read-tree ${s.greenTree} && GIT_INDEX_FILE=$d/i git checkout-index -a -f; rc=$?; rm -rf "$d"; exit $rc`;
    const r = await sh(cwd, restore, 60_000);
    const after = await sh(cwd, s.greenCmd, 3 * 60_000);
    s.restored = r.code === 0;
    trace.event("restore_green", { tree: s.greenTree, cmd: s.greenCmd, restore_rc: r.code, passes_after: after.code === 0 });
  }
}

// qh's updatePlan: one "- [status] step — note" line per step, replaced by step name.
function planKey(l: string): string {
  let x = l;
  const i = x.indexOf("] ");
  if (i >= 0) x = x.slice(i + 2);
  const j = x.indexOf(" — ");
  if (j >= 0) x = x.slice(0, j);
  return x.trim().toLowerCase();
}

export function updatePlan(cwd: string, p: { step?: string; status?: string; note?: string }): string {
  if (!p.step) return "ERROR missing_arg: update_plan needs step";
  const path = join(cwd, ".qh", "PLAN.md");
  let data = "";
  try {
    data = readFileSync(path, "utf8");
  } catch {}
  let lines = data.replace(/\n+$/, "").split("\n");
  if (lines.length === 1 && lines[0] === "") lines = [];
  let entry = `- [${p.status ?? ""}] ${p.step}`;
  if (p.note) entry += " — " + p.note;
  let replaced = false;
  lines = lines.map((l) => {
    if (planKey(l) === planKey(entry)) {
      replaced = true;
      return entry;
    }
    return l;
  });
  if (!replaced) lines.push(entry);
  const content = lines.join("\n") + "\n";
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, content);
  return "OK plan updated:\n" + content;
}
