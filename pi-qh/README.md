# pi-qh: qh it2 as a pi extension

`pi-qh` ports the behaviour of qh's best harness (`harness/it2`) to [pi](https://pi.dev) as a single
extension. It also adds a **chunked execution** mode for requests too big for one context.

```bash
pi -e pi-qh/extensions/qh.ts --thinking medium --model <provider/model> "task..."
QH_CHECK="go test ./..." pi -e pi-qh/extensions/qh.ts ...       # check on finish
QH_PI_CHUNKS=1 pi -e pi-qh/extensions/qh.ts ...                  # chunked execution
```

It is built only from pi's public extension API (no fork, no patches), with no runtime dependencies beyond
what pi provides (`typebox`, node built-ins).

Offline tests drive the real pi binary against a scripted stub server: `node pi-qh/test/run.mjs`. There are
10 scenarios and 37 checks, one or more per mechanism.

## Mechanisms (defaults = it2)

| qh it2 mechanism | pi events used | notes |
|---|---|---|
| **L1 think cap** (8K tokens; keep the partial reasoning; next turn with thinking off) | `message_update` (thinking deltas) → `ctx.abort()`; `agent_settled` → `sendMessage(…, {triggerTurn})`; `before_provider_request` | See the note below this table. |
| **Check on finish** (`QH_CHECK`, 4 attempts) | `agent_before_settle` → feedback entry + `continue` | A text reply is pi's "finish". |
| Output-limit recovery, empty reply | `agent_before_settle` | qh's nudge texts. Plain pi stops on a truncated reply; that is how it lost b06. |
| Budget: 80 turns / 60 min | `turn_start` → `ctx.abort()` | — |
| **L2 build/test nudge** after 300 untested written lines | `tool_result` (write) | Reset by a build/test bash run (qh's test-command regex). |
| **L3 restore last green tree** on a budget stop | `tool_result` (passing bash test) snapshots via a throwaway git index; `agent_settled` restores | — |
| Fuzzy edit (whitespace-tolerant, re-indents) | `tool_call` (edit) rewrites `oldText`/`newText` | Port of qh's `fuzzyReplace`. |
| Verify writes (gofmt / JSON) | `tool_result` | — |
| Loop detector (3 repeats in 8 calls; 14 calls without progress) | `tool_result` | — |
| Output cap 4K tokens + spill to `.qh/spill/` | `tool_result` | Applied after pi's own 2000-line/50 KB truncation. |
| Read window 400 lines; bash timeout 120 s (max 600) | `tool_call` mutates the input | Plain pi's bash has no default timeout. |
| `update_plan` tool (`.qh/PLAN.md`) | `registerTool` | — |
| qh system prompt and task card (task, workspace, check, file tree) | `before_agent_start`, `input` | — |

The L1 think cap needs two workarounds:
- pi drops aborted assistant messages, and never sends an assistant message that has neither text nor tool
  calls. So the cut reasoning is re-inserted, exactly as qh sends it (empty content plus
  `reasoning_content`), before each act nudge on every later request. That keeps the history append-only
  and the prefix cache valid.
- Thinking is switched off with `chat_template_kwargs.enable_thinking=false`, plus Qwen's non-thinking
  sampling, which leaves the cache untouched. pi's `setThinkingLevel` would change `reasoning_effort`,
  which is part of the cached prefix on this server.

Configuration:
- `QH_PI_CONFIG=<json>`: override any key in `src/config.ts`.
- `QH_PI_OFF=thinkCap,loops,…`: switch mechanisms off for ablations.
- `QH_PI_TRACE`, `QH_PI_STATUS`: JSONL trace and final status file, used by `qh eval --agent "eval/runner/pi_agent.sh qh"`.

## Chunked execution (`QH_PI_CHUNKS=1`)

The goal is to avoid compaction instead of surviving it. A big request is split into chunks, and every chunk
runs with a fresh, small context:

1. **Plan phase.** The model orients itself, then calls `plan_chunks` with 3–12 chunks. Each chunk has a
   title, a goal and an optional `done_when` shell command.
2. **Chunk k.** The `context` event rewrites every request to:
   - system prompt and tools;
   - the task card (a stable, cached prefix);
   - a chunk card: a `[done]/[this]/[later]` plan fence, the notes ledger and this chunk's goal;
   - only this chunk's own messages.
   
   `chunk_done(report)` runs `done_when`; if it fails, the chunk stays open (up to 3 attempts). On success the
   report is appended to `.qh/NOTES.md`, the plan in `.qh/CHUNKS.md` is updated, and the next chunk starts
   fresh. A chunk that runs past `chunkMaxTurns` is told to hand over.
3. **Final phase.** The normal finish and acceptance check run.

Ideas taken from [late-cli](https://github.com/mlhher/late-cli):
- one fresh context per step;
- only a short report flows back;
- workers stop and report.

Ideas taken from [pi-task](https://github.com/mjasnikovs/pi-task):
- a checkbox plan as the cursor;
- the step fence;
- verbatim notes kept by the host;
- the repository as the memory.

It differs from both: it runs inside one pi process, with no child processes, and every request keeps the
same system/tools/task-card prefix. That matters on a server with slow prefill and a prefix cache.

## Differences from native qh it2 (known)

- **Warm-up.** qh's prefix warm-up request is not ported. The first turn of a run is a cold prefill.
- **Tools.** pi's `read/bash/edit/write` plus `update_plan` replace qh's
  `read_file/write_file/edit_file/run/grep/list_dir/delegate/finish`.
  - There are no `grep`/`list_dir` tools; the model uses bash.
  - There is no read-only sub-agent. It was never used in it1/it2 runs.
  - pi's tool descriptions and edit format (an `edits[]` array) are pi's own.
- **Finish.** Finishing is a text reply, not a `finish` tool call. Empty replies are nudged; a text reply
  counts as a finish attempt.
- **Compaction.** It is pi's own, without qh's checkpoint and git state. It never triggered on Tier B
  (contexts stay under 74K tokens). Chunk mode is the answer for long tasks.
- **Terminal-Bench.** Containers are not supported yet: the tools run on the host. pi exposes
  `createBashTool` operations and tool overrides, so docker-backed tools are possible.
