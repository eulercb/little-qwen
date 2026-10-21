# little-qwen

`qh` is a small Go agent harness built to get the most out of **Qwen3.8-27B** (served remotely through an
OpenAI-compatible MLX server) and to compensate for its weak spots. It also includes an evaluation suite
and a Harness²-style improvement loop.

- **Plan / background:** [`docs/BUILD_PLAN.md`](docs/BUILD_PLAN.md)
- **Measured endpoint behaviour:** [`docs/ENDPOINT.md`](docs/ENDPOINT.md)
- **Progress log (start here when resuming):** [`STATUS.md`](STATUS.md)
- **Experiment log (Propose → Probe → Compose records):** [`docs/EXPERIMENTS.md`](docs/EXPERIMENTS.md)

## Layout
```
cmd/qh/                 CLI: run, eval, report, digest, validate, smoke
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
