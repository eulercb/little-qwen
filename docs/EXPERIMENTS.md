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
