# Experiment log (Harness² Propose → Probe → Compose)

Every harness change gets an entry. Rules (from docs/BUILD_PLAN.md):
- An edit must cite a concrete trajectory moment (run dir + turn) — no speculative edits.
- Prefer levers in this order: skill → script → sub-agent → Go hook (config.json / loop) → always-on text.
  If always-on text grows, cut something else.
- The editor never reads hidden tests (`eval/tasks/*/hidden`, `solution/`) or hidden-check output while
  proposing edits; evidence comes from `qh digest`, transcripts and final diffs.
  (Caveat: in this project the same agent wrote the tasks, so the leak rule is only partially enforceable.
  Held-out tasks written later mitigate this.)
- Validate on tasks not used to derive the edit; reject edits that regress >1 task.

Template:
```
## EXP-<n>: <title>   (status: proposed | probing | adopted | rejected)
- Evidence: <run dir, turn numbers, what happened>
- Hypothesis:
- Candidates: INV = ..., DOM = ...
- Probe: <results dirs, numbers>
- Decision + why:
- Library entry: library/<domain>/<name>/EXPERIENCE.md
```

---

## Evidence notes from it0-baseline (collected while running)
- **E1 (a04-cli-args r1, turns 15–19):** two `edit_file` calls failed with `no_match`. The model's `old_text` had
  5 leading tabs where the file had 4. `read_file` renders lines as `%5d\t<content>`, so in tab-indented Go the
  separator tab looks like an indentation tab. The model recovered with `sed -n '74p' | cat -A` and `sed -i`,
  costing 3 extra turns (~1.5 min).
- **E2 — RETRACTED.** My probe printed the parsed config with `%+v`, which shows nil and empty slices
  identically as `[]`; after the hidden tests were fixed, a04 r1 passes all hidden tests including the nil-slice
  check, so the model's slices *were* nil. No evidence remains for `review_before_finish`; its probe was dropped.
  Lesson for the editor: verify a claimed defect with a check that can distinguish the cases.
  Original note: **(a04-cli-args r1, deliverable):** prompt says "`Tags` is nil when no tag is given … `Args` is nil when
  there are none"; the delivered parser initialises both to empty non-nil slices. Visible check passed, so the
  harness accepted `finish`. Class F7 (spec detail dropped). Candidate lever: `review_before_finish`.
- **Eval fix:** a04 hidden test demanded the literal `-vx` in the error for `-vx`; the model's `unknown flag -x`
  is a reasonable reading of the prompt, so the test now accepts either.
- **Eval fix:** a08 baseline 0.00 was a helper-name collision (`fakeClock`) between hidden and agent tests;
  all hidden helpers are now `zzh*`-prefixed and self-contained.
- **E3 (a10-topk-words r1, turn 3):** one turn produced 7,804 tokens of which 7,692 were reasoning
  (799 s ≈ 13 min at ~10 tok/s under 2× load) before two tool calls — designing the apostrophe/streaming
  tokenizer in its head. Class F4-ish (runaway thinking at `medium`). Candidate lever: a streaming thinking
  budget (`think_budget_tokens`): abort the turn when reasoning exceeds N tokens before any action, append a
  "decide and act" hint, and let the next turn run unbudgeted. Prompt was cached, so the retry costs only the hint.
- **E4 (a10-topk-words r1, turn 4):** prefill 7,890 tokens / 202 s — the 7.7K reasoning tokens generated in
  turn 3 are sent back (`preserve_thinking`) but generated tokens are never cached, so they are re-prefilled,
  then occupy context for the rest of the run. Thinking is 32% of all completion tokens in it0 Tier A.
  Candidate (Phase-5-style ablation): `strip_reasoning` — never send reasoning back, consistently from the
  first turn (prefix stays stable). Risk: losing the model's own earlier reasoning may hurt continuity.
- **E5 (a10-topk-words r1, turns 10–19+):** the visible check passed at 15:08, 15:15 and 15:19, yet the
  executor kept refining (buffer-boundary handling, bigger tests, then a 106-line reference-oracle test)
  instead of finishing. Cost: 15+ min. Unclear yet whether this extra verification raises hidden-test
  success (it may — the task has subtle rules) — judge by outcome before acting.
- **Ablation A1 (Phase 5): `reasoning_effort=low`** for all turns (harness/cand/e5-low). Motivation: thinking is
  32% of completion tokens in it0 and decode dominates wall time; Tier A success is saturated, so the question is
  whether low keeps success while cutting time. Published AA agentic index drops medium→low (44.4→38.9), so expect
  some loss on harder tasks.
- **E6 (h02-config-merge r1, held-out baseline, score 0.92):** spec: "for `Tags`: a non-nil override slice replaces
  base, even if empty". Delivered `out.Tags = append([]string(nil), override.Tags...)`, which yields nil for an empty
  override. Visible check passed; the model's own tests never exercised the empty-override clause. (Eval fix at the
  same time: the hidden test also demanded no aliasing of base slices, which the prompt does not require — removed.)
- **E7 (h04-mini-template r1, held-out baseline, score 0.92):** spec: quoted filter arguments support `\"` and `\\`.
  Deliverable splits the tag on `|` before parsing quotes, so `default:"a | b"` fails ("unterminated argument").
  Again an explicit clause with no test of its own.
- **Cluster C1 (F7, spec clause untested → silently wrong):** E6, E7. Revives the `review_before_finish` lever
  (checklist of every explicit requirement + where it is implemented + which test covers it). Probe on h02, h04
  (×2 each, now evidence tasks) and validate on spec-heavy b01, b06 (base numbers from the Tier-B baseline).

## EXP-1: whitespace-tolerant edit_file (E1)   (status: adopted as safe hook, no contrast signal)
- Probe eval/results/exp1-fuzzy (a04, a05, h01, h02 ×1): the fallback never fired — no `no_match` errors occurred.
  `no_match` is rare (3 in 16 base runs). Success/turn differences vs base are within single-run noise.
- Decision: adopt `fuzzy_edit: true` into the composed harness as a mechanical safety net (it only changes behaviour
  when an edit would otherwise fail; unit-tested re-indentation). `read_line_sep` (changes every read) not adopted;
  its probe was dropped.

## EXP-3: thinking budget 3,000 tokens (E3)   (status: REJECTED)
- Probe eval/results/exp3-think (a10, a12, h03, h04 ×1). The budget fired once per task, always at turn 2 (the
  design turn), after ~150 s. The next (unbudgeted) turn re-derived the design from scratch:
  aborted+next thinking vs base's largest thinking turn — a10 3.0K+6.6K vs 7.7K, a12 3.0K+0.8K vs 2.5K,
  h03 3.0K+5.5K vs 9.9K, h04 3.0K+3.5K vs 4.5K. Worse in 3/4: the discarded partial reasoning is pure waste.
- Success 3/4 → 4/4 (h04 0.92 → 1.00) is within single-run variance; wall comparisons are confounded for a10/a12
  (base ran at 2× load).
- Lesson: design-turn thinking on these tasks is mostly necessary work; cutting it moves it, not removes it.
  If revisited, keep the partial reasoning (e.g. continue from it) rather than discarding it.
- OpenCode comparator (eval/results/opencode-baseline, h01): success, 9.2 min vs qh base 3.2 min (both single-stream).

## EXP-4: strip_reasoning (E4)   (status: REJECTED, n=1)
- Probe eval/results/exp4-strip (a04, a05, a10, a12, h03, h04 ×1).
- Prefill fell 55% (114.8K → 51.4K tokens) as predicted, but thinking rose (52.9K → 60.7K); on the two
  load-comparable tasks it nearly doubled (h03 11.3K → 21.8K, h04 13.0K → 21.2K) and wall time got worse
  (16.3 → 28.6, 19.9 → 27.6 min). Without its earlier reasoning in context the model re-derives each turn, and a
  thinking token costs ~4× a prefill token on this server. Success 5/6 in both arms (a04 1.0 → 0.75, h04 0.92 → 1.0).
- Keep `preserve_thinking` (default). Note: Tier-A base wall times for a01–a12 were measured at 2× load, so a
  single-stream base rerun (it0-base-p1) was queued for time comparisons.

## A1: reasoning_effort=low   (status: REJECTED — counter-intuitive result)
- Probe eval/results/exp5-low (a03, a05, a07, a09, a10, a12, h02, h04 ×1).
- Thinking tokens **doubled** (44.4K → 94.0K; higher on 7/8 tasks, e.g. a10 15.4K → 41.6K, a03 0.5K → 6.5K);
  completion 84K → 144K; total wall 133 → 149 min (base partly at 2× load, so time is only indicative).
  Success 6/8 → 7/8 (a07 1.0 → 0.93, h02/h04 0.92 → 1.0) — within single-run noise.
- On this server/template, `low` is not a cheap mode; `medium` stays the default. (Untested: `enable_thinking:false`.)
- **A2/A3 queued:** `thinking_mode` = `off` (Qwen non-thinking sampling: T=0.7, top_p=0.8, presence=1.5) and
  `adaptive` (thinking on turns 1–2 and after a turn with a tool error / failing test or check / no tool call).
  Verified first that toggling thinking keeps the prefix cached.
- **Single-stream base (eval/results/it0-base-p1, a03 a04 a05 a07 a09 a10 a12):** 7/7, wall 6.7 ± 2.7 min,
  7.7K completion (2.8K thinking), 12 turns. Against it, A1 (low) on the 6 shared tasks: 37.6 → 111.9 min,
  thinking 15.2K → 75.2K — confirms the rejection.

## A2: thinking off (all turns)   (status: REJECTED)
- Probe eval/results/exp6-think-off vs single-stream base (it0-base-p1 + it0-baseline h02/h04).
- Easy tasks ~2× faster (a03 4.7→2.5, a05 3.0→1.4, a09 7.0→3.1 min) but a07, a10, a12, h04 hit max_turns (80);
  score 7.83 → 5.24, success 6/8 → 4/8, turns 97 → 368, tool errors 10 → 37.
- Trajectories (a10, h04): shotgun debugging — dozens of 5–15 s turns of `go test … | grep … ; echo done`
  probes, 14–15 loop warnings per run, no hypothesis forming. Thinking is where this model plans/debugs.
- Note for A3 (adaptive): many failing probes end in `| head; echo done` and exit 0, so a "bad turn" detector based
  on exit codes misses them; loop warnings are a better trigger.

## A3: adaptive thinking   (status: REJECTED)
- Probe eval/results/exp7-think-adaptive (same 8 tasks; one run redone after a container restart).
- Some easy tasks faster (a03 4.7→2.8, a07 6.4→3.9 min) but a12 and h04 hit max_turns (80): score 7.83 → 7.36,
  wall 59.8 → 95.9 min, turns 97 → 230. Non-thinking turns degrade without producing tool errors (the "bad turn"
  trigger rarely fires), so the agent drifts before thinking is re-enabled.
- Conclusion of the thinking ablations (A1–A3, EXP-3, EXP-4): for this model, keep thinking on at `medium`
  for every turn and keep the reasoning in context. Time savings must come from elsewhere (cache discipline,
  fewer re-prefills, fewer wasted turns), not from cutting thinking.

## L1: think_cap_continue, little-coder's thinking budget adapted   (status: PROBE QUEUED)
- Source: little-coder's thinking-budget extension. When reasoning exceeds the budget, it keeps the partial
  reasoning as context and retries with thinking off. See docs/LITTLE_CODER.md.
- Evidence (trace scan of 1429 qh turns):
  - 4 turns hit `max_tokens` 16K with all-thinking and no action: b05 1211 s, b04 705 s, b01 766 s, exp4/h03
    1016 s. On b05 the next turn also had to re-prefill the 16K tokens, and the task timed out at 60 min.
  - The model was drafting the full implementation in its reasoning, not looping (unique-line ratio 0.64–0.78).
  - 9 of 1429 turns exceeded 8K.
- Candidate `harness/cand/l1-think-cap`: `think_budget_tokens: 8000` plus `think_cap_continue: true`. The partial
  reasoning is kept as an assistant message, a harness nudge says "act now, don't draft code in reasoning", and
  the next turn runs with `enable_thinking:false` (cache-neutral).
  - Differences from EXP-3: the reasoning is kept, not discarded, and the cap is 8K instead of 3K.
- Update 22:05: b06 baseline failed (wall clock, 73.7 min, only 7 turns). Turns 2, 5, 6 and 7 each spent 16K
  tokens on thinking (~17–21 min each) with no action, and each was re-prefilled on the next turn.
  - Tier-B baseline: b01–b03 solved, b04 0.925, b05 and b06 failed.
  - Both failures are runaway thinking; b06 was added to the probe.
- Probe `eval/results/expL1-think-cap`: b01, b04, b05, b06, h03, a10 ×1, against it0-baseline.
