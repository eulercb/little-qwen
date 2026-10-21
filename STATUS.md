# STATUS — resumable progress log

> Read this first if you are picking up this project (human or agent).
> Keep it current: update "Current state" and append to "Log" at every milestone, then commit.

## Goal
Build `qh`, a Go agent harness that gets the most out of **Qwen3.8-27B** (remote MLX server, OpenAI-compatible)
and compensates for its weaknesses, then improve it with the Harness² method (Propose → Probe → Compose,
evidence-gated, lesson library). Guiding doc: `docs/BUILD_PLAN.md` (Part 4 is the roadmap).

## How to resume
1. `go build -o bin/qh ./cmd/qh` (Go 1.24, stdlib only).
2. Endpoint: `$REMOTE_LLM_ENDPOINT` (Cloudflare quick tunnel; URL changes when the tunnel restarts).
   Check: `curl -s $REMOTE_LLM_ENDPOINT/v1/models`. See `docs/ENDPOINT.md` for measured behaviour.
3. Validate the harness: `./bin/qh validate harness/base`.
4. Run evals: `./bin/qh eval --tasks eval/tasks --harness harness/base --out eval/results/<iter> [--only tierA] [--reps 1]`
   (resumable: runs with an existing result.json are skipped). Then `./bin/qh report eval/results/<iter>`.
5. Inspect a run: `./bin/qh digest eval/results/<iter>/<task>/r1`.
6. Continue from "Next steps" below.

## Current state (update me)
- Phase 0 DONE: endpoint characterised (docs/ENDPOINT.md); smoke 49/50 structured tool calls (eval/results/phase0).
- Phase 1 DONE (first version): Go loop, 9 tools, output cap+spill, repair, loop detector, traces, finish→check.
- Phase 2 PARTIAL: compaction implemented (model checkpoint + git state) but not yet exercised by a task.
- Eval suite: 12 Tier-A (a01–a12), 4 Tier-B (b01–b04), 2 held-out (h01–h02). All validated with
  `eval/runner/validate_tasks.sh` (hidden fails on start, passes with solution/).
- Baseline it0 (harness/base) running/ran on Tier A: see eval/results/it0-baseline/summary.md.
- Experiments: EXP-1 (E1 edit indentation) candidates in harness/cand/e1-sep and harness/cand/e1-fuzzy — to probe.
- OpenCode comparator: `npm install opencode-ai` into a scratch dir, then
  `OPENCODE_BIN=<dir>/node_modules/.bin/opencode ./bin/qh eval --agent eval/runner/opencode_agent.sh ...`
  (models.opencode.ai is blocked by the egress proxy; OPENCODE_DISABLE_MODELS_FETCH=true works around it).
- Not possible here: Tier C (Terminal-Bench needs Docker; daemon not running), Tier D (PWA spec not provided).

## Next steps
1. Finish it0 baseline (Tier A), run Tier B + held-out baseline.
2. Probe EXP-1 candidates on edit-heavy tasks (a04, a05, h01, h02); adopt if contrast supports.
3. OpenCode baseline on Tier A subset + Tier B.
4. Phase 3: planner + per-step fresh contexts if Tier-B traces show context/planning failures.

## Log
- 2026-10-21 14:00 — endpoint probed; harness skeleton; smoke test 49/50.
- 2026-10-21 14:05 — Tier-A baseline it0 started (parallel 2). First results: 4/4 success.
- 2026-10-21 14:35 — Tier-B tasks b01–b04, held-out h01–h02 added; EXP-1 candidates implemented (config knobs
  `read_line_sep`, `fuzzy_edit`; defaults keep old behaviour).
