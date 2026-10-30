# little-qwen

`qh` is a small Go agent harness built to get the most out of **Qwen3.8-27B** (served remotely through an
OpenAI-compatible MLX server) and to compensate for its weak spots. It also includes an evaluation suite
and a Harness²-style improvement loop.

- **Plan / background:** [`docs/BUILD_PLAN.md`](docs/BUILD_PLAN.md)
- **Measured endpoint behaviour:** [`docs/ENDPOINT.md`](docs/ENDPOINT.md)
- **Progress log (start here when resuming):** [`STATUS.md`](STATUS.md)
- **Experiment log (Propose → Probe → Compose records):** [`docs/EXPERIMENTS.md`](docs/EXPERIMENTS.md)

## Results so far (2026-10-22)

Snapshot; experiments are paused — resume with `eval/runner/resume.sh <tunnel-url>` (see STATUS.md); full records in [`docs/EXPERIMENTS.md`](docs/EXPERIMENTS.md), raw data in `eval/results/`.
All runs are single repetitions unless stated, so per-task differences of a few minutes are within noise.

### The model + server, measured
| | finding | consequence for the harness |
|---|---|---|
| Speed | cold prefill ~80–140 tok/s, decode ~25–50 tok/s (10× slower prefill than the plan assumed) | cache discipline is the #1 performance property |
| Prefix cache | reuses up to the previous prompt; **generated tokens are never cached** | append-only context; big outputs are paid twice (decode + re-prefill) |
| `reasoning_effort` | rendered into the prompt prefix — switching it breaks the cache | one effort per context segment; side requests reuse the segment's effort |
| Thinking on/off | only changes the prompt suffix — cache-neutral | per-turn thinking policies are possible (but see below) |
| Concurrency | 2 parallel agents ≈ same total throughput as 1 | evals run one at a time |
| Tool calling | 49/50 single-tool prompts → correct structured call (98%) | repair layer kept as a safety net; 0 repairs needed so far |

### Baseline harness (`harness/base`)
| suite | result |
|---|---|
| Tier A (12 small Go tasks, hidden tests) | **12/12** solved; single-stream mean **6.7 min**, 12 turns, 7.7K output tokens per task |
| Held-out (4 tasks) | 2/4 fully correct, mean score **0.96** (two runs missed one spec clause each) |
| Tier B (6 multi-step tasks) | **3/6 solved**. b01 46.7 min, b02 19.9 min, b03 4.7 min, b04 0.925. b05 and b06 failed on the 60-min limit, both because of runaway turns that spent all 16K output tokens on thinking. With L1 (an 8K thinking cap that keeps the reasoning, from little-coder), b05 was solved in 33 min, and the Tier-B mean on b01, b04, b05, b06 rose from 0.48 to 0.74. Adding L2 (a nudge to build and test after large untested writes) gave b01 1.00 in 29 min, b05 1.00 in 30 min, and b06 0.96, the first non-zero b06 result from any harness, pi and little-coder included. Both are composed into `harness/it1`. Its full run: Tier-B mean 0.818 (base 0.654, pi 0.828, little-coder 0.650) and held-out 4/4 at 1.00 (base 0.96). it2 (it1 + L3 restore-last-green) scored a Tier-B mean of 0.988, with b06 at 0.96; pooled with it1, qh scores 0.903 on Tier B |
| Tier D: Caça-Palavras PWA (9 milestone sessions, hidden checker with 39 items incl. Playwright e2e) | it2: **38/39 (0.97)** in 118 min; the one miss is a click on the completion card |
| Tier D as ONE session (d01s, no task-given milestones) | pi + pi-qh **chunk mode: 38/39** in 139 min. The model planned 10 chunks, each with a fresh small context, and there was **no compaction** (largest prompt 62K). The same harness without chunks scored 33/39, with 1 compaction and prompts up to 109K |
| vs OpenCode, pi and little-coder (same model/server) | On 7 Tier-A/held-out tasks: qh 0.976 in 57.3 min, little-coder 0.980 in 84.2 min, vanilla pi 0.996 in 54.9 min, so about tied. Stock OpenCode scored 0.776 in 143.9 min. On Tier B, qh it1 + it2 pooled score 0.903, **pi + pi-qh (qh it2 ported as a pi extension) 0.885**, plain pi 0.828 and little-coder 0.650. See [docs/LITTLE_CODER.md](docs/LITTLE_CODER.md) |

Infrastructure features verified with the real model: re-prefill events **0** across baseline runs (prefix stays
stable); prefix warm-up makes a task's first turn hit ~1.9K cached tokens (8 s instead of 20–50 s); compaction
writes a useful checkpoint as a cache hit and the new segment resumes from its "Next steps".

### Harness experiments (Harness² Propose → Probe → Compose)
| experiment | idea | outcome |
|---|---|---|
| EXP-1 fuzzy `edit_file` | tolerate indentation mismatches (seen when the model copied a separator tab) | **adopted as safety net** — rare failure, no measurable effect |
| EXP-3 thinking budget | abort runaway thinking turns (one turn thought for 13 min) | **rejected** — the next turn re-derives the same plan, net cost higher |
| EXP-4 strip reasoning | don't send old thinking back (less prefill) | **rejected** — prefill −55%, but thinking nearly doubles; slower overall |
| A1 effort `low` | cheaper thinking | **rejected** — counter-intuitively *doubles* thinking; ~3× slower |
| A2 thinking off | no thinking at all | **rejected** — easy tasks 2× faster, hard tasks flail into the 80-turn cap (score 7.83 → 5.24) |
| A3 adaptive thinking | think on early turns and after errors | **rejected** — drifts without visible errors; 2 tasks hit the turn cap |
| C1 spec-clause review / `go-pitfalls` skill | catch silently-missed spec clauses (both held-out misses) | **queued (paused)** |

**Takeaways so far.** Qwen3.8-27B is strong on well-specified Go tasks: every Tier-A task and the first three
Tier-B tasks are solved. The harness's main contributions are speed (cache-stable context, warm-up, no
concurrency) and reliability (retries over a flaky tunnel, acceptance check on `finish`, compaction).
All five attempts to save time by reducing thinking made things worse: this model plans and debugs in its thinking,
so the default (`medium`, thinking every turn, reasoning kept in context) stays. The remaining failure pattern is
**explicit spec clauses that the model never tests** (an empty-but-non-nil slice, a `|` inside a quoted argument),
which is what the queued C1 experiments target.

Not possible in this environment: Terminal-Bench (Tier C, needs a Docker daemon) and the PWA task (Tier D, spec
not provided). Several eval bugs found along the way were fixed and rescored (hidden-test helper name collisions,
over-strict checks); see the "Eval fix" notes in the experiment log.

## Layout
```
cmd/qh/                 CLI: run, eval, report, digest, validate, smoke, rescore, propose
internal/llm/           streaming OpenAI-compatible client (retries, idle watchdog, usage + cache metrics)
internal/loop/          append-only agent loop, finish→acceptance check, compaction, delegation
internal/tools/         read_file, write_file, edit_file, run, grep, list_dir (+ update_plan, delegate, finish in loop)
internal/repair/        tool-call parse repair (Qwen3-Coder XML / Hermes JSON), JSON arg repair, loop detection
internal/ctxmgr/        output cap + spill files, deterministic checkpoint state
internal/trace/         JSONL traces + transcripts, failure taxonomy F1–F10
internal/harness/       Harness²-shaped component tree loader + validator
internal/eval/          task runner, hidden scoring, summaries, editor digests
harness/base/           the editable harness (system prompt, guardrails, memory, tool guidance, skills, sub-agents, config.json)
eval/tasks/             tasks: prompt.md, repo/ (start), hidden/ (scoring tests), solution/ (reference)
eval/results/<iter>/    per-iteration results, traces and summary.md
library/                adopted harness pieces + EXPERIENCE.md lessons (Harness² library)
```

## Quick start
```bash
go build -o bin/qh ./cmd/qh
export REMOTE_LLM_ENDPOINT=https://<tunnel>.trycloudflare.com
./bin/qh validate harness/base
./bin/qh smoke                                    # 50 single-tool prompts
./bin/qh run --dir /path/to/repo --prompt "Add a --json flag to ..." --check "go test ./..."
./bin/qh eval --out eval/results/it1 --only tierA --reps 1
./bin/qh report eval/results/it1
./bin/qh digest eval/results/it1/a01-lru-cache/r1
eval/runner/validate_tasks.sh                     # check that tasks are solvable and hidden tests discriminate
eval/runner/resume.sh https://<tunnel>            # resume the paused experiment queue (see STATUS.md)
```

## Design in one paragraph
The prompt is an append-only log: the system message (always-on harness text) is identical for every task and
sub-agent, the task card is written once, and nothing earlier is ever edited, so each request is a strict
extension of the previous one and the server's prefix cache (which can't rewind a hybrid model's recurrent
state) covers everything but the new tail. Tool outputs are capped (overflow goes to `.qh/spill/` files that the
model can page through), writes are verified immediately (gofmt/JSON), malformed tool calls are repaired, loops are
detected, and "done" is not trusted: `finish` runs the acceptance check and sends failures back. When the prompt
gets large the model writes a checkpoint and the context restarts from the stable prefix + task card +
checkpoint + git state. Broad read-only investigations can be delegated to fresh-context sub-agents.
