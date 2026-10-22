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
6. Long runs go through the queue: append `qh eval ...` lines to `eval/queue.txt`, then
   `nohup eval/runner/queue.sh >> eval/queue.log 2>&1 &` (finished lines move to `eval/queue.done`).
   The queue keeps a pidfile (`eval/queue.pid`); a second instance refuses to start. Never use
   `pgrep -f <pattern>` in a wait loop: it matches the loop's own shell and never exits.
7. Compare a candidate with base: `eval/runner/compare.py eval/results/it0-baseline eval/results/<exp>`;
   time/tool breakdown: `eval/runner/analyze.py eval/results/<dir>`.
8. Re-score after fixing a hidden test: `./bin/qh rescore eval/results/<dir>` (needs the workdirs in /tmp/qh-runs).
9. Qwen as editor (Phase 6): `./bin/qh propose --out harness/cand/<name> eval/results/<dir>/<task>/r1 ...`.
10. Continue from "Next steps" below.

## Current state (update me)
- Harness `qh` complete through Phase 2 (+ Phase 6 tooling `qh propose`). Base harness: harness/base.
- Endpoint facts that drive the design: docs/ENDPOINT.md (generated tokens never cached; effort is part of the
  cached prefix but thinking on/off is not; no concurrency gain; prefix warm-up works).
- Eval suite: Tier A a01–a12, Tier B b01–b06, held-out h01–h04 (all validated by eval/runner/validate_tasks.sh).
- Baselines (harness/base): Tier A 12/12 (it0-baseline, 2× load); held-out h01–h04 mean 0.96 (h02/h04 0.92);
  single-stream Tier-A rerun in it0-base-p1 (queued/running); Tier B queued.
- Experiments (docs/EXPERIMENTS.md): EXP-1 fuzzy_edit adopted as safety net; EXP-3 think budget REJECTED;
  EXP-4 strip_reasoning REJECTED; A1 effort=low REJECTED (doubles thinking); E2 retracted.
  Pending: A2 thinking off, A3 adaptive thinking, C1 (review_before_finish vs go-pitfalls skill), OpenCode baseline.
- The eval queue (eval/queue.txt) runs in the background via eval/runner/queue.sh; it commits results itself.
  If the session restarted, check `test -f eval/queue.pid && kill -0 $(cat eval/queue.pid)`; if not running and
  queue.txt has lines, restart it with `nohup eval/runner/queue.sh >> eval/queue.log 2>&1 &`.

## Next steps
1. When A2/A3 finish: compare with it0-base-p1 (+ it0-baseline for h02/h04). Adopt if success holds and time drops.
2. When Tier B + C1 probes finish: decide C1 (INV review vs DOM skill vs both).
3. Compose harness/it1 = base + adopted edits (fuzzy_edit, + winners); run full suite (Tier A+B+held-out) and
   OpenCode comparison; write up results in README.
4. Phase 6: run `qh propose` (Qwen as editor) on failing runs; compare its proposals with the human-editor ones.
5. More/harder tasks if Tier B saturates.

## Log
- 2026-10-21 14:00 — endpoint probed; harness skeleton; smoke test 49/50.
- 2026-10-21 14:05 — Tier-A baseline it0 started (parallel 2). First results: 4/4 success.
- 2026-10-21 14:35 — Tier-B tasks b01–b04, held-out h01–h02 added; EXP-1 candidates implemented (config knobs
  `read_line_sep`, `fuzzy_edit`; defaults keep old behaviour).
- 2026-10-21 22:40 — EXP-3/4 and A1 rejected; thinking on/off found cache-neutral; A2/A3 queued.
- 2026-10-22 ~01:30 — container restarted (processes lost, files kept); queue restarted, exp7 resumed.
