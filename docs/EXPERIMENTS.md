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
