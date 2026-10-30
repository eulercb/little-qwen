# STATUS — resumable progress log

> Read this first if you are picking up this project (human or agent).
> Keep it current: update "Current state" and append to "Log" at every milestone, then commit and push.

## Goal
Build `qh`, a Go agent harness that gets the most out of **Qwen3.8-27B** (remote MLX server, OpenAI-compatible)
and compensates for its weaknesses, then improve it with the Harness² method (Propose → Probe → Compose,
evidence-gated, lesson library). Guiding doc: `docs/BUILD_PLAN.md` (Part 4 is the roadmap).
Results summary for readers: README.md "Results so far". Full experiment records: docs/EXPERIMENTS.md.

## ⏸ PAUSED (2026-10-30 10:49 UTC) on request; nothing is running, everything is committed

Resume with `eval/runner/resume.sh https://<tunnel>.trycloudflare.com`. The queue continues with:
1. qh `harness/it2-long` on d01s, the third arm of the single-session stress test. It was interrupted 26 min
   in and discarded.
2. pi + pi-qh on the 7 Tier-A/held-out tasks.
3. The Terminal-Bench subset with it2 (3/20 done), then with little-coder.

Headline:
- Tier-B mean: qh it1 0.818, it2 0.988 (pooled 0.903); **pi + pi-qh 0.885**; base 0.654, pi 0.828,
  little-coder 0.650.
- d01s (the PWA brief as ONE session): **pi-qh chunks 38/39 with 0 compactions** vs pi-qh without chunks
  33/39 (1 compaction, prompts up to 109K). qh it2 with 9 task-given sessions: 38/39.
- 7 Tier-A/held-out tasks: qh, pi and little-coder are about tied (0.98–1.00); OpenCode scored 0.776 at 2.5× the
  time.
- Tier D (d01 PWA): it2 38/39.
- Tier C (it2, partial): cancel-async-tasks 1, cobol-modernization 1, configure-git-webserver 0.

### pi-qh: qh it2 as a pi extension (2026-10-29, user request)
- `pi-qh/` (README there) ports every it2 mechanism to pi's public extension API, plus a new chunked mode
  (`QH_PI_CHUNKS=1`) that avoids compaction.
  - Design inputs: late-cli (fresh context per step) and pi-task (checkbox plan, notes ledger), both cloned
    under /home/user/{mlhher,mjasnikovs}.
- Offline tests: `node pi-qh/test/run.mjs` (10 scenarios, 37 checks; real pi binary + scripted stub).
  The full eval path was also checked offline.
- Eval wiring:
  - `eval/runner/pi_agent.sh qh|qh-chunks`.
  - `qh eval --agent` now passes QH_CHECK, QH_RAW_PROMPT_FILE and QH_AGENT_OUT, and reads agent_status.json.
  - Task specs gained `base` + `prompt_suffix`. `eval/tasks/d01s-caca-palavras-single` is the d01 brief as one
    session.
- Not yet: Terminal-Bench for pi-qh (needs docker-backed tool overrides).

### Direction (2026-10-29, discussed with the user while the tunnel was down)
- OpenCode is dropped from future comparisons. Its 7-task result stays as a record.
- Proposed next steps, pending the user's choice:
  1. Long-context stress test: d01 as ONE session (the whole brief, about a 4 h budget), so compaction actually
     fires. Compare with the 9-session run.
     - In 163 real runs compaction never triggered; the largest context was 73.5K tokens against an 88K
       threshold.
  2. Port it2 as a pi extension package, then A/B it against qh it2 on Tier B.
     - Mechanisms to port: the L1 think cap, the L2 build/test nudge, L3 restore-green, fuzzy edit,
       check-on-finish, recovery from a max-token truncation, and a wall-clock budget.
     - pi already replays reasoning_content and keeps the context append-only, which is likely why vanilla pi
       matched the qh base.
  3. Finish Tier C: it2 is at 3/20, then little-coder.

### Resume in one command
```bash
eval/runner/resume.sh https://<new-tunnel>.trycloudflare.com      # add --no-opencode to skip the comparator install
```
It builds `bin/qh`, runs the offline tests, checks the endpoint (model list + a chat round-trip), installs the
comparator agents (OpenCode into `~/.cache/qh-opencode`; pi and little-coder into `~/.cache/qh-pi` and
`~/.cache/qh-littlecoder`), prints the remaining queue and starts the background queue.
If it stops at the endpoint check: the tunnel is down, or the server needs `--allowed-host <tunnel host>` (403).
Then follow "Decisions waiting on those results" below as items finish.

### Where things stand
- Paused on request at ~04:30; resumed 12:18 with tunnel `massachusetts-refresh-bringing-cubic`, which died at 12:33
  (Cloudflare 530 "Origin DNS error"); paused again 14:25. Runs that failed only because of the outage (b04, b05)
  were discarded. All finished results are committed and pushed.
- Remaining queue (`eval/queue.txt`, in order; `eval/queue.done` lists what finished):
  1. (done) L1 think-cap probe.
  2. (done) pi on Tier B: mean 0.828, the same as qh + L1 (0.825); b06 failed on a 16K thinking truncation.
  3. little-coder on Tier B.
  3b. (done) L2 probe: b01 1.00 / 29.3, b05 1.00 / 30.3, b06 0.96. ADOPTED.
  3c. little-coder Tier-B b04–b06 again (outage re-run).
  3d. (done) it1: Tier B 0.818, held-out 1.00.
  4. (done) Base rep 2 on h02/h04: both 1.00, so C1 is noise and its probes were retired.
  4b. it2 (it1 + L3 restore_green) on Tier B (`eval/results/it2`).
  4. Base rep 2 on h02/h04.
  5. C1 INV probe (`e2-review`).
  6. C1 DOM probe (`c1-dom`).
  7. Broader OpenCode baseline.
  8. d01 (Tier D).
  9. Tier C subset baseline (`tbC-base`).
  10. little-coder on the Tier C subset (`tbC-littlecoder`).
- Outage handling (added at the second pause): `qh eval` aborts the remaining runs when the endpoint is unavailable
  (exit≠0); the queue waits for `$REMOTE_LLM_ENDPOINT/health` before each item and keeps failed items to retry them
  (caveat: an item failing for a non-endpoint reason retries every minute — fix or delete that line).

### pi / little-coder comparison and ideas: added 2026-10-22 ~21:00 (user request)
- Write-up: docs/LITTLE_CODER.md covers the wiring, the mechanism-by-mechanism mapping to qh with our trace
  evidence, and the verdicts. The little-coder clone used for the study is at /home/user/itayinbarr/little-coder
  (commit 89d4fa0).
- Comparators: `eval/runner/pi_agent.sh` (vanilla pi) and `"eval/runner/pi_agent.sh little"` (little-coder).
  Both are queued on the same 7 single-session tasks as OpenCode, writing to `eval/results/{littlecoder,pi}-baseline`.
- Candidate L1 `harness/cand/l1-think-cap` adds `think_budget_tokens: 8000` and `think_cap_continue: true`. It is
  little-coder's thinking budget with the partial reasoning kept and one thinking-off act turn.
  - Motivation: runaway all-thinking turns hit the 16K output limit on b01, b04 and b05 (12–20 min each; b05 then
    timed out).
  - Probe queued: `eval/results/expL1-think-cap` on b01, b04, b05, h03 and a10. Compare with it0-baseline, and
    read the `think_budget` trace events.
- `eval/runner/tool_stats.py <result dirs>` gives per-run overwrite, unread-edit and edit-failure stats.
- Tier C bridge: `qh tbench --agent eval/runner/lc_tb_agent.py` (validated with a scripted stub). It is queued
  last, as `tbC-littlecoder`, and needs the clone at /home/user/itayinbarr/little-coder (resume.sh clones it).
- Pending: revisit the bg-shell/ShellSession ideas after the Tier C baseline.

### Tier C (Terminal-Bench 2) and Tier D (Caça-Palavras PWA) — added 2026-10-22 evening
- Docker works: `eval/runner/docker-up.sh` starts dockerd with the `mirror.gcr.io` registry mirror (Docker Hub
  answers 429 to anonymous pulls from this egress IP).
- Tier C runner: `./bin/qh tbench --only @eval/tbench/subset.txt --out eval/results/<name>` (`--oracle` runs the
  reference solution). Tasks come from the public repo laude-institute/terminal-bench-2 @ 2fd12b8 cloned to
  /home/user/laude-institute/terminal-bench-2 (Harbor's registry backend is blocked; not needed). qh's tools run
  inside the task container (static `bin/qh-static` copied in as qh-tool). Containers get host networking, proxy env
  and the proxy CA; apt is switched to https. Egress: `*.debian.org` was enabled by the user; `astral.sh` is still
  blocked, so the runner provisions uv/uvx from PyPI (`eval/runner/tb-cache.sh`) before the official verifier.
  Agent timeout = task timeout × 4 (`--timeout-mult`, the local model is slow) — a documented deviation.
- Validated: oracle reward 1.0 on the reference solutions (see eval/results/tb-oracle); a scripted-agent run through
  qh's container tools also scored 1.0. `build-cython-ext` fails even with the oracle (dependency drift) — excluded.
- Tier D task: `eval/tasks/d01-caca-palavras` (brief from the user, verbatim). 9 sessions (M0–M8), one fresh qh
  context per milestone in the same workdir; visible check `npm run check`; hidden checker scores 39 items
  (static rules, npm check, hidden vitest tests, PWA artifacts, Playwright e2e at 360×740). Reference solution 39/39,
  empty repo 0/39. Run: `./bin/qh eval --only d01-caca-palavras --out eval/results/<name>`.

### Manual operation (what resume.sh automates)
- Endpoint: `export REMOTE_LLM_ENDPOINT=https://<tunnel>` (the queue and qh read it from the environment).
- OpenCode: `export OPENCODE_BIN=<path to opencode>` before starting the queue (its line reads
  `${OPENCODE_BIN:-opencode}`); `eval/runner/opencode_agent.sh` disables the models fetch (models.opencode.ai is
  blocked by the egress proxy) and bounds the run with `timeout`.
- Start: `nohup eval/runner/queue.sh >> eval/queue.log 2>&1 &` — one item at a time, commits+pushes results per item;
  `qh eval` skips finished runs and re-runs runs with status `error`.
- Running? `test -f eval/queue.pid && kill -0 $(cat eval/queue.pid) && echo running`.
  Stop: `kill $(cat eval/queue.pid); kill $(cat eval/.eval.lock)`; then delete the half-finished run
  (`eval/results/<out>/.inprogress/` or a result with status `error`). The unfinished line stays in queue.txt.
- **Never** use `pgrep -f <pattern>` or a `/proc` cmdline scan for a pattern inside a shell loop: the loop's own
  shell matches (this killed/hung watchers twice). Use the pid/lock files.
- Waiting for results: a background loop on `wc -l < eval/queue.done`; background commands stop after 2 h, so re-arm.

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
- 2026-10-22 12:33 — tunnel died (530 Origin DNS error); 14:25 queue stopped, b04/b05 outage runs discarded;
  added outage abort in qh eval + health gate/keep-on-failure in queue.sh.
- 2026-10-22 ~14:40 — session wrapped up: added eval/runner/resume.sh (one-command resume), STATUS rewritten.
- 2026-10-22 ~18:50 — Tier C runner + Tier D task built and validated (docker enabled, oracle sweep running).
- 2026-10-22 18:46 — new tunnel magnitude-academic-technique-imagine; server needs --allowed-host for it; queue started and waiting on /health.
- 2026-10-22 ~21:30 — Tier-B: b04 0.925, b05 failed (wall clock 60 min; a 20-min runaway thinking turn), b06 running.
  - Studied little-coder and pi (docs/LITTLE_CODER.md).
- 2026-10-22 22:02 — Tier-B baseline done: b01–b03 solved, b04 0.925, b05 and b06 failed on wall clock.
  - Both failures were runaway 16K-token thinking turns (b06 had 4 of them).
  - The L1 probe now includes b06.
- 2026-10-23 00:22 — comparators on 7 Tier-A/held-out tasks:
  - qh: 0.976 in 57.3 min.
  - little-coder: 0.980 in 84.2 min (h03 hit the 45-min cap).
  - vanilla pi: 0.996 in 54.9 min.
  - About tied. Tier-B comparisons were queued to test whether qh's design pays off on long tasks.
- 2026-10-23 ~01:25 — the container restarted mid-L1-probe; a10 was done, and the b01 partial run was discarded.
  - Restarted with resume.sh.
  - docker-up.sh now removes stale pidfiles left by a restart (dockerd refused to start because the pidfile's pid
    now belonged to another process).
- 2026-10-23 03:31 — PAUSED on request during the L1 probe (b05 run discarded). Resume with eval/runner/resume.sh.
- 2026-10-23 19:55 — resumed on tunnel nursing-seats-tab-pioneer. Its first /health returned a Cloudflare 502;
  it was up 20 s later.
- 2026-10-23 21:36 — L1 probe done and ADOPTED for it1 (b05 fail → 1.00 in 33 min; Tier-B mean 0.48 → 0.74).
  - b06 still fails: late testing plus whole-file rewrites. Candidate L2 proposed (docs/EXPERIMENTS.md).
  - pi Tier-B started.
- 2026-10-27 00:20 — the container had restarted, so the queue was dead (pi Tier-B b01 was in progress and was
  discarded). Resumed on tunnel auctions-dot-journalist-region.
- 2026-10-27 04:10 — the container restarted again during little-coder Tier-B b01 (partial run discarded).
  Resumed with resume.sh.
- 2026-10-27 05:45–06:00 — tunnel outage.
  - little-coder Tier-B b04–b06 "finished" in 0–9 min with Cloudflare error pages as output. Those results were
    deleted and re-queued.
  - Fix: external-agent runs now check for outages (a Cloudflare page in the output, or a failing /health) and
    record status error "endpoint unavailable". That makes qh eval and qh tbench stop early, and the queue keep
    the item.
  - little-coder Tier-B valid so far: b01 timeout (60 min, 0), b02 1.00 / 25.7, b03 1.00 / 8.7.
- 2026-10-27 08:02 — L2 probe done and ADOPTED: b06 0.96, the first non-zero b06 from any harness.
  - Composed harness/it1 = base + fuzzy_edit + L1 + L2; queued it1 on Tier B + held-out.
- 2026-10-27 08:36 — PAUSED on request during the little-coder b04 re-run (interrupted run discarded).
- 2026-10-28 — resumed on tunnel msg-opportunities-campbell-suddenly.
- 2026-10-28 04:11 — little-coder Tier-B done: mean 0.650 (b01 0, b02 1, b03 1, b04 0.90, b05 1, b06 0; four
  tasks hit the 60-min limit). Compare pi 0.828 and qh base 0.654. it1 run started.
- 2026-10-28 08:46 — it1 done.
  - Tier-B mean 0.818. b06 was cut mid-edit with a broken build; L3 "restore last green" proposed.
  - Held-out 4/4 at 1.00.
- 2026-10-28 09:04 — base rep 2: h02 and h04 both 1.00, so C1 is retired as noise (probes removed from the queue).
  it2 Tier-B started.
  - The Tier D (d01) and Tier C (Terminal-Bench subset) items now run with harness/it2 (outputs `it2`,
    `tbC-it2`) instead of base. qh's best version is what we compare with little-coder, and these tiers are too
    slow to run twice.
- 2026-10-28 09:13 — PAUSED on request during it2 Tier-B b01 (interrupted run discarded).
- 2026-10-29 — resumed on tunnel rosa-spokesman-beaver-frederick.
- 2026-10-29 05:03 — it2 Tier-B: mean 0.988 (b04 0.97, b06 0.96, the rest 1.00); L3 never fired.
  - Pooled it1 + it2: 0.903. Next: the OpenCode comparison, then d01 and Tier C with it2.
- 2026-10-29 07:18 — OpenCode on the 7 tasks: 0.776 in 143.9 min (qh 0.976 / 57.3, pi 0.996 / 54.9,
  little-coder 0.980 / 84.2). d01 (Tier D) with it2 started.
- 2026-10-29 09:16 — d01 with it2: 38/39 = 0.97 (9 sessions, 118 min).
  - The first score of 0 was a harness bug: QH_TASK_DIR was relative. Fixed, and rescored with `qh rescore`.
  - Tier C (Terminal-Bench subset) with it2 started.
- 2026-10-29 11:37 — PAUSED on request during tbC-it2 (3/20 done; interrupted db-wal-recovery discarded).
- 2026-10-30 01:09 — resumed on tunnel controllers-yellow-dare-hamburg.
  - pi-qh live smoke: a05 1.00 / 5.1 min.
  - pi-qh Tier B: b01 1.00 / 37.6, b02 1.00 / 27.9, b03 1.00 / 7.5. The think cap fired and worked live in b01
    and b02.
- 2026-10-30 03:15 — the container restarted during pi-qh b04 (run discarded). Resumed with resume.sh.
- 2026-10-30 05:37 — pi-qh Tier B done: mean 0.885 (b04 0.93, b06 0.38, the rest 1.00); native qh pooled
  0.903. The d01s single-session stress test started (piqh-chunks first).
- 2026-10-30 07:56 — d01s with pi-qh chunks: 38/39 in 138.7 min (10 chunks, 0 compactions, largest prompt 62K),
  the same as it2's 9-session result. pi-qh without chunks started.
- 2026-10-30 10:23 — d01s with pi-qh WITHOUT chunks: 33/39 in 146 min (1 compaction, prompt up to 109K).
  Chunks win, 38 vs 33. qh it2-long started.
- 2026-10-30 10:49 — PAUSED on request during it2-long d01s (26 min in, discarded).
  - `qh tbench` now stages runs in .inprogress, like qh eval.
  - Added the pi/little-coder comparators and candidate L1 (think_cap_continue), and queued them after Tier B.
