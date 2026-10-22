# STATUS — resumable progress log

> Read this first if you are picking up this project (human or agent).
> Keep it current: update "Current state" and append to "Log" at every milestone, then commit and push.

## Goal
Build `qh`, a Go agent harness that gets the most out of **Qwen3.8-27B** (remote MLX server, OpenAI-compatible)
and compensates for its weaknesses, then improve it with the Harness² method (Propose → Probe → Compose,
evidence-gated, lesson library). Guiding doc: `docs/BUILD_PLAN.md` (Part 4 is the roadmap).
Results summary for readers: README.md "Results so far". Full experiment records: docs/EXPERIMENTS.md.

## ▶ Experiments RESUMED (2026-10-22 12:18 UTC) — how to resume after a pause

The queue was paused at ~04:30 and resumed at 12:18 UTC with a new tunnel URL (check `eval/queue.pid` to see
whether it is still running). All finished results are
committed. The interrupted Tier-B baseline run (b04, which was ~33 min in) was deleted, so it reruns from scratch.

**Resume in a fresh session/container:**
1. Build: `go build -o bin/qh ./cmd/qh` (Go 1.24, stdlib only; `bin/` is gitignored).
2. Endpoint: `export REMOTE_LLM_ENDPOINT=https://<tunnel>.trycloudflare.com` (Cloudflare quick tunnel; the URL
   changes whenever the tunnel restarts, and the server must allow that host or it answers 403).
   Check: `curl -s $REMOTE_LLM_ENDPOINT/v1/models` should list `mlx-community/Qwen3.8-27B-4bit`.
3. Sanity: `go test ./internal/...` and `./bin/qh validate harness/base`.
4. OpenCode comparator (only needed for the last queue item): install it anywhere, e.g.
   `mkdir -p /tmp/oc && cd /tmp/oc && npm install opencode-ai@latest`, then `export OPENCODE_BIN=/tmp/oc/node_modules/.bin/opencode`
   **before** starting the queue (the queue line reads `${OPENCODE_BIN:-opencode}`). The wrapper
   `eval/runner/opencode_agent.sh` sets `OPENCODE_DISABLE_MODELS_FETCH=true` (models.opencode.ai is blocked by the
   egress proxy) and bounds the run with `timeout` (opencode run sometimes doesn't exit).
5. Start the queue: `nohup eval/runner/queue.sh >> eval/queue.log 2>&1 &`
   - It runs `eval/queue.txt` top to bottom, one at a time, and commits+pushes `eval/results` after each item.
   - Remaining items (in order): (1) Tier-B baseline (b01–b03 done → runs b04, b05, b06); (2) base rep 2 on h02/h04;
     (3) C1 INV probe `harness/cand/e2-review`; (4) C1 DOM probe `harness/cand/c1-dom`; (5) broader OpenCode baseline.
   - Expect roughly 6–10 hours in total (Tier-B tasks take 5–50 min each; probes are 2 reps × 4 tasks).
   - `qh eval` resumes: runs with a finished result.json are skipped; runs whose status is `error`
     (endpoint outage) are re-run.
6. Is it running? `test -f eval/queue.pid && kill -0 $(cat eval/queue.pid) && echo running`.
   Stop it: `kill $(cat eval/queue.pid); kill $(cat eval/.eval.lock)` (the unfinished queue line stays in queue.txt;
   delete the half-finished run dir it was writing, see eval/results/<out>/.inprogress or a result with status error).
   **Never** use `pgrep -f <pattern>` or a `/proc` cmdline scan for a pattern inside a shell loop: the loop's own shell
   matches the pattern (this killed/hung watchers twice). Use the pid/lock files.
7. Waiting for results without polling: a background shell loop on `wc -l < eval/queue.done` works; background
   commands are stopped after 2 h, so re-arm per queue item.

**Decisions waiting on those results (do these next):**
1. Tier B baseline complete → update README table (b04–b06).
2. C1: compare `eval/results/exp2-review` and `eval/results/exp2-dom` with base (`it0-baseline`, h02/h04 r1+r2, b01, b06)
   using `eval/runner/compare.py`. Adopt the INV (`review_before_finish: true`) and/or DOM (`skills/go-pitfalls`)
   candidate if it fixes the spec-clause misses without regressions on b01/b06; record in docs/EXPERIMENTS.md.
3. Compose `harness/it1` = `harness/base` + adopted edits (`fuzzy_edit: true` already decided, + C1 winner(s)).
   Run the full suite on it1 (Tier A, Tier B, held-out) single-stream; compare with base; update README.
4. Write the Harness² library entries (`library/<domain>/<name>/EXPERIENCE.md`) for adopted edits.
5. Phase 6: `./bin/qh propose --out harness/cand/qwen-<name> <run dirs>` (Qwen as editor) on the failing runs
   (e.g. h02/h04 base) and compare its proposal with the C1 candidates.
6. If Tier B saturates (all solved), write harder tasks (validate each with `eval/runner/validate_tasks.sh`).

## Tool reference
- Run evals: `./bin/qh eval --harness <dir> --out eval/results/<name> [--only ids|tiers] [--reps N] --parallel 1`
  (always `--parallel 1`: the server has no concurrency gain). Report: `./bin/qh report eval/results/<name>`.
- Inspect a run: `./bin/qh digest [--thoughts 300] eval/results/<name>/<task>/r1` (never reads hidden output).
- A/B contrast: `eval/runner/compare.py <base-dir> <cand-dir>`; time/tool breakdown: `eval/runner/analyze.py <dir>`.
  Single-stream base for Tier-A times: `eval/results/it0-base-p1` (+ `it0-baseline` h0x). `it0-baseline` a01–a12
  were run at 2× load: compare their success/tokens, not their wall times.
- Re-score after fixing a hidden test: `./bin/qh rescore eval/results/<dir>` (needs the workdirs under /tmp/qh-runs;
  they do not survive a new container — rerun instead).
- Validate tasks: `eval/runner/validate_tasks.sh [ids]` (hidden fails on the start repo, passes with solution/).
- Hidden-test conventions: helpers must be `zzh*`-prefixed and self-contained (agent tests may declare any name);
  test only what the prompt states.
- Harness knobs live in `<harness>/config.json` (see `internal/harness/harness.go` Config for the full list).

## Current state
- Harness `qh`: Phases 0–2 done (streaming client, append-only loop, 9 tools, cap+spill, repair, loop detector,
  finish→acceptance check, compaction verified with the real model, prefix warm-up, read-only sub-agents),
  Phase 6 tooling (`qh propose`, `harness/editor`). Offline loop tests: `go test ./internal/...`.
- Endpoint facts driving the design: docs/ENDPOINT.md (generated tokens never cached; `reasoning_effort` is part of
  the cached prefix, thinking on/off is not; no concurrency gain; warm-up works).
- Eval suite: Tier A a01–a12, Tier B b01–b06, held-out h01–h04 — all validated.
- Baselines (harness/base): Tier A 12/12 (single-stream mean 6.7 min); held-out mean 0.96 (h02, h04 = 0.92);
  Tier B b01–b03 solved (46.7 / 19.9 / 4.7 min), b04–b06 pending; OpenCode h01: solved in 9.2 min vs qh 3.2 min.
- Experiments: EXP-1 fuzzy_edit ADOPTED (safety net); EXP-3 think budget, EXP-4 strip reasoning, A1 effort=low,
  A2 thinking off, A3 adaptive thinking all REJECTED; E2 retracted. Pending: C1 (INV review / DOM skill).
- Candidate harness dirs: `harness/cand/*` (rejected ones kept for reproducibility; `test-compact` is a sanity
  harness with a 9K compaction threshold).

## Log
- 2026-10-21 14:00 — endpoint probed; harness skeleton; smoke test 49/50.
- 2026-10-21 14:05 — Tier-A baseline it0 started (parallel 2). First results: 4/4 success.
- 2026-10-21 14:35 — Tier-B tasks b01–b04, held-out h01–h02 added; EXP-1 candidates implemented.
- 2026-10-21 22:40 — EXP-3/4 and A1 rejected; thinking on/off found cache-neutral; A2/A3 queued.
- 2026-10-22 ~01:30 — container restarted (processes lost, files kept); queue restarted, exp7 resumed.
- 2026-10-22 ~02:40 — A2/A3 rejected; thinking ablations concluded. Tier-B baseline started.
- 2026-10-22 ~04:30 — experiments PAUSED on request (b01–b03 done; b04 interrupted and discarded).
- 2026-10-22 12:18 — resumed with new tunnel URL (massachusetts-refresh-bringing-cubic); queue restarted at Tier-B b04.
