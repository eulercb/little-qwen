// Chunked execution: finish a large request without context compaction.
//
// Ideas taken from late-cli (fresh context per step, only a short report flows
// back, the planner never carries a worker's transcript) and pi-task (a
// checkbox plan on disk as the cursor, a [done]/[this]/[later] fence,
// host-kept verbatim notes; the repository is the memory). Unlike both, it
// runs inside ONE pi process: the `context` event rewrites what the model sees
// on every request, so a chunk's request is
//
//   [system + tools]  [task card (stable prefix, cached)]  [chunk card: plan
//   fence + notes ledger + this chunk's goal]  [this chunk's messages only]
//
// Earlier chunks' messages stay in pi's session file but are never sent again,
// so the context stays small and compaction never triggers.
//
// Flow: plan phase (orient, then plan_chunks) → chunk 1..n (each ends with
// chunk_done, which runs the chunk's done_when command) → final phase (the
// normal finish + acceptance check).
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import type { QhConfig } from "./config.ts";
import type { Trace } from "./trace.ts";

interface Chunk {
  title: string;
  goal: string;
  doneWhen?: string;
  done: boolean;
  report?: string;
  attempts: number;
}

export interface Chunks {
  planInstruction(): string;
  setTaskCard(card: string): void;
  /** A nudge when the model tries to finish while chunks remain. */
  pendingNudge(): string | undefined;
}

const MAX_REPORT_CHARS = 2500;

export function registerChunks(pi: ExtensionAPI, cfg: QhConfig, trace: Trace): Chunks {
  let card = "";
  let chunks: Chunk[] = [];
  let current = -1; // -1: plan phase; chunks.length: final phase
  let boundaryId = ""; // tool call id whose result closes the previous phase
  let chunkTurns = 0;
  let planNotes = "";

  const phase = () => (current < 0 ? "plan" : current >= chunks.length ? "final" : "chunk");

  const persist = (cwd: string) => {
    try {
      mkdirSync(join(cwd, ".qh"), { recursive: true });
      const plan = chunks.map((c, i) => `- [${c.done ? "x" : " "}] ${i + 1}. ${c.title}${c.doneWhen ? `  (done when: \`${c.doneWhen}\`)` : ""}\n  ${c.goal}`).join("\n");
      writeFileSync(join(cwd, ".qh", "CHUNKS.md"), plan + "\n");
      writeFileSync(join(cwd, ".qh", "NOTES.md"), notesLedger() + "\n");
    } catch {}
  };

  const notesLedger = () => {
    let out = planNotes ? `### From orientation\n${planNotes}\n` : "";
    chunks.forEach((c, i) => {
      if (c.done && c.report) out += `### ${i + 1}. ${c.title}\n${c.report}\n`;
    });
    return out.trim();
  };

  const chunkCard = () => {
    let b = "\n\n# Chunked execution (harness)\nThis task runs in chunks; each chunk starts with a fresh context. Your memory between chunks is the repository itself, .qh/CHUNKS.md and the notes below. Look at the current files rather than assuming what earlier chunks did.\n\n## Plan\n";
    chunks.forEach((c, i) => {
      const mark = c.done ? "done" : i === current ? "this" : "later";
      b += `[${mark}] ${i + 1}. ${c.title}\n`;
    });
    const notes = notesLedger();
    if (notes) b += `\n## Notes from earlier chunks\n${notes}\n`;
    if (phase() === "final") {
      b += "\n## Final step\nAll chunks are done. Run the acceptance check and the tests, fix any remaining problem, then finish with a short summary (a reply without a tool call).\n";
      return b;
    }
    const c = chunks[current];
    b += `\n## Current chunk: ${current + 1}. ${c.title}\nGoal: ${c.goal}\n`;
    if (c.doneWhen) b += `Done when: \`${c.doneWhen}\` exits 0 (chunk_done runs it).\n`;
    b +=
      "Work ONLY on this chunk; do not start later chunks. Build and test as you go. When the chunk is done and verified, call chunk_done with a short report for the next chunks: files created or changed, interfaces they need (names, signatures, formats), how to verify, and anything left undone.\n";
    if (chunkTurns > cfg.chunkMaxTurns) {
      b += `\nCHUNK BUDGET EXHAUSTED (${chunkTurns} turns): call chunk_done now with an honest report of what is done and what remains.\n`;
    }
    return b;
  };

  pi.on("turn_start", async () => {
    if (phase() === "chunk") chunkTurns++;
  });

  pi.on("context", async (event) => {
    if (phase() === "plan" || !boundaryId) return;
    const msgs = event.messages as any[];
    const idx = msgs.findIndex((m) => m.role === "toolResult" && m.toolCallId === boundaryId);
    if (idx < 0) return;
    const head = { role: "user", content: [{ type: "text", text: card + chunkCard() }], timestamp: Date.now() };
    return { messages: [head, ...msgs.slice(idx + 1)] as any };
  });

  pi.registerTool({
    name: "plan_chunks",
    label: "plan_chunks",
    description:
      "Split the task into ordered chunks (call once, after orienting). Each chunk later runs with a fresh context, so make each one a coherent slice (one module, feature or milestone) completable in about 10-25 tool calls.",
    parameters: Type.Object({
      chunks: Type.Array(
        Type.Object({
          title: Type.String({ description: "Short chunk name" }),
          goal: Type.String({ description: "What to build or change in this chunk: concrete files, functions, behaviour (1-4 sentences)" }),
          done_when: Type.Optional(Type.String({ description: "Shell command that exits 0 once the chunk is done (e.g. a focused test run); omit if none" })),
        }),
        { minItems: 1, maxItems: 20 },
      ),
      notes: Type.Optional(Type.String({ description: "Key facts from orientation that later chunks need (conventions, commands, constraints)" })),
    }),
    async execute(id: string, params: any, _signal: any, _onUpdate: any, ctx: any) {
      if (phase() !== "plan") return { content: [{ type: "text", text: "ERROR: the plan already exists; work on the current chunk and call chunk_done." }], details: undefined };
      chunks = (params.chunks ?? []).map((c: any) => ({ title: String(c.title), goal: String(c.goal), doneWhen: c.done_when?.trim() || undefined, done: false, attempts: 0 }));
      planNotes = String(params.notes ?? "").slice(0, MAX_REPORT_CHARS);
      current = 0;
      chunkTurns = 0;
      boundaryId = id;
      persist(ctx.cwd);
      trace.event("chunks_planned", { n: chunks.length, titles: chunks.map((c) => c.title) });
      return { content: [{ type: "text", text: `OK ${chunks.length} chunks planned; starting chunk 1 with a fresh context.` }], details: undefined };
    },
  } as any);

  pi.registerTool({
    name: "chunk_done",
    label: "chunk_done",
    description: "Finish the current chunk with a short report for the next chunks. Runs the chunk's done_when command first; if it fails, the chunk stays open.",
    parameters: Type.Object({
      report: Type.String({ description: "Files changed, interfaces later chunks need, how to verify, anything left undone" }),
    }),
    async execute(id: string, params: any, _signal: any, _onUpdate: any, ctx: any) {
      if (phase() !== "chunk") return { content: [{ type: "text", text: phase() === "plan" ? "ERROR: call plan_chunks first." : "ERROR: all chunks are done; run the acceptance check and finish." }], details: undefined };
      const c = chunks[current];
      c.attempts++;
      let suffix = "";
      if (c.doneWhen) {
        const r = await pi.exec("bash", ["-c", c.doneWhen], { cwd: ctx.cwd, timeout: 10 * 60_000 });
        const code = r.killed ? 124 : r.code;
        trace.event("chunk_check", { chunk: current + 1, exit: code, attempt: c.attempts });
        if (code !== 0) {
          const out = ((r.stdout ?? "") + (r.stderr ?? "")).slice(-3000);
          if (c.attempts < 3 && chunkTurns <= cfg.chunkMaxTurns) {
            return { content: [{ type: "text", text: `ERROR done_when failed: \`${c.doneWhen}\` exited ${code}; the chunk is not done. Fix it, then call chunk_done again.\n${out}` }], details: undefined };
          }
          suffix = ` (done_when \`${c.doneWhen}\` still failing at hand-over)`;
        }
      }
      c.done = true;
      c.report = String(params.report ?? "").slice(0, MAX_REPORT_CHARS) + suffix;
      trace.event("chunk_done", { chunk: current + 1, turns: chunkTurns, attempts: c.attempts, forced: !!suffix });
      current++;
      chunkTurns = 0;
      boundaryId = id;
      persist(ctx.cwd);
      const next = phase() === "final" ? "all chunks are done; final step next" : `starting chunk ${current + 1} with a fresh context`;
      return { content: [{ type: "text", text: `OK chunk ${current} done; ${next}.` }], details: undefined };
    },
  } as any);

  return {
    planInstruction: () =>
      "\n# Chunked execution (harness)\nThis task is large, so it runs in chunks, each with a fresh context. First orient briefly (read the spec and the relevant files; do not implement anything yet), then call plan_chunks with 3-12 ordered chunks. Each chunk should be a coherent slice (one module, feature or milestone) completable in about 10-25 tool calls, with a concrete goal and, when possible, a done_when shell command that exits 0 once it is done. If the task already defines milestones, follow them.\n",
    setTaskCard: (t: string) => {
      card = t;
    },
    pendingNudge: () => {
      if (phase() === "plan") return "You have not planned the chunks yet. Orient, then call plan_chunks.";
      if (phase() === "chunk") return `Chunk ${current + 1} (${chunks[current].title}) is still open. Continue it and call chunk_done when it is done; do not finish the whole task yet.`;
      return undefined;
    },
  };
}
