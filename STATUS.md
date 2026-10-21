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

## Current state
- Phase 0 (baseline/instrumentation): IN PROGRESS
