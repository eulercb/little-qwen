# pi and little-coder: comparators and ideas

[little-coder](https://github.com/itayinbarr/little-coder) (studied at commit 89d4fa0, v1.20.0) is
[pi](https://pi.dev) (`@earendil-works/pi-coding-agent`) plus about 30 extensions and 30 skill markdown
files, tuned for small local models. It is the closest public project to qh.

Its published results:
- Aider Polyglot: 45.6% with Qwen3.5-9B, against 19.1% for vanilla Aider on the same model.
- Terminal-Bench 2.0: 24.6% ± 3.2 with Qwen3.6-35B-A3B.

Its whitepaper is *Honey, I Shrunk the Coding Agent*. Its per-mechanism numbers are in
`docs/benchmark-reproduction.md` of that repo.

This file covers three things:
1. How we run pi and little-coder as comparators.
2. What each little-coder mechanism maps to in qh.
3. What our own traces say about whether each idea would help here.

Little-coder's ideas were tuned on 9B–35B-A3B models with a 32K context. Our model is a dense 27B with a 262K
context, served at ~30 tok/s decode. So every idea is checked against qh's own data before anything is
adopted, following the Harness² rule.

## Comparators

`eval/runner/pi_agent.sh` is an external agent for `qh eval --agent`:
- `eval/runner/pi_agent.sh` runs vanilla pi: 4 tools (read, bash, edit, write), a ~2.8K-char system prompt.
- `"eval/runner/pi_agent.sh little"` runs little-coder: 26 tools, a ~7.8K-char system prompt plus ~12.8K
  chars of tool schemas, and per-turn skill cards appended at the tail.

Both get the same provider definition:
- `openai-completions` API at `$REMOTE_LLM_ENDPOINT/v1`, model `mlx-community/Qwen3.8-27B-4bit`.
- `contextWindow` 131072, `maxTokens` 16384.
- `--thinking medium`, which sends `reasoning_effort: medium`, the same as qh.

little-coder registers the provider its own way. It sends a `developer` role, `max_completion_tokens` and
`store:false`; the splash server accepts all three (checked). It runs with
`LITTLE_CODER_PERMISSION_MODE=accept-all`, which is what its benchmark runner uses. It falls back to its
`default_model_profile`: thinking budget 4096 and temperature 0.3, i.e. the out-of-the-box experience.

Config lives in a throwaway directory (`PI_CODING_AGENT_DIR`, `XDG_CONFIG_HOME`), so nothing lands in the
scored workdir. `eval/runner/resume.sh` installs both packages into `~/.cache/qh-pi` and
`~/.cache/qh-littlecoder` (Node ≥ 22.19).

Gotchas found while wiring them up:
- `pi -p` reads stdin until EOF when stdin is not a TTY. Under `qh eval` that is a silent hang with no request
  ever sent, so the wrapper passes `</dev/null`.
- Node's fetch ignores `HTTPS_PROXY`, so the wrapper sets `NODE_USE_ENV_PROXY=1` (container egress goes through
  a proxy).
- The endpoint is single-stream, so a comparator must never run while the queue is running. Run them only as
  queue items.
- `qh eval --agent` runs one prompt; it does not replay multi-session tasks (`sessions`, e.g. b02 and d01).
  Compare on single-session tasks only.

Terminal-Bench: little-coder's harbor adapter (`benchmarks/harbor_adapter`) drives `pi --mode rpc` on the
host. It exposes only `ShellSession`/`ShellSessionCwd`/`ShellSessionReset`, proxied into the task container
(a stateful shell: cwd kept, output trimmed to 200 lines, duplicate lines collapsed). Running it under
`qh tbench` would need a bridge to `docker exec`, which isn't built. Its published 24.6% on the full TB2 (89
tasks, a different model) is not comparable to our 20-task subset.

Queued (see `eval/queue.txt`), on the same 7 single-session tasks as the OpenCode comparison:
`eval/results/littlecoder-baseline` and `eval/results/pi-baseline`.

## Mechanisms, mapped to qh, with our evidence

The evidence comes from `eval/runner/tool_stats.py` and trace analysis over the 68 qh runs in `eval/results`
(as of 2026-10-22).

| little-coder mechanism | qh equivalent | Our data | Verdict |
|---|---|---|---|
| **write-guard**: `write` refuses existing files and returns the exact `edit` call shape. In the paper it fired on 57% of Polyglot exercises with the 9B model and was the main gain. | `write_file` overwrites and says "overwrote". | 27 of 69 `write_file` calls overwrote a file, mostly stubs. 8 overwrote a file of more than 40 lines, and those runs passed. Decode cost of big-file overwrites: ~0.1 min/run. | Not adopted: no harm seen for the 27B. Re-check on Tier D (9 sessions on growing files). |
| **read-guard-edit**: `edit` refuses unread files. | none | 0 of 229 `edit_file` calls targeted a file not read or written earlier in the run. | Rejected (never fires). |
| **read-guard**: trims a read to 30 lines when it would overflow the window. | `read_file` returns 400 lines per call; tool output cap of 4K tokens with spill files. | No context overflows in our runs. | Already covered. |
| **thinking-budget**: when reasoning passes the budget, abort, keep the partial reasoning as context, and retry with thinking off. Fired ~0.9×/exercise. | `think_budget_tokens` (EXP-3) discarded the partial reasoning; rejected. | **Runaway thinking is real on Tier B.** 4 turns hit the 16K output limit with all-thinking and no action: b05 (1211 s, then a 16K re-prefill; the task timed out at 60 min), b04 (705 s), b01 (766 s), h03/exp4 (1016 s). Reading those traces, the model drafts the whole implementation inside its reasoning, then has to decode it again as tool arguments. 9 turns were over 8K, out of 1429. | **Candidate L1**, `think_cap_continue` (below). |
| **skill-inject**: 80–150-token tool cards picked per turn by error > recency > intent, appended at the conversation tail so the prefix stays cached. | Always-on tool guidance in the prefix; structured tool errors with recovery hints; Harness² skills. | 9 `edit_file` failures in 229 (4%): 6 `no_match`, 3 `missing_arg`, each recovered on the next turn. | Low value now. The **tail-injection** principle is already how qh adds hints and nudges (append-only). |
| **knowledge-inject**: algorithm cheat-sheets scored against the prompt. | DOM skills (C1 `go-pitfalls`) | C1 probe queued (exp2-dom). | Same idea as our DOM lever; no change. |
| **quality-monitor**: empty response, unknown tool, verbatim repeat, malformed args. | Nudges, loop detector, XML/JSON tool-call repair. | — | Already covered. |
| **turn-cap** (TB: 40 turns) and the compaction watchdog at 80%. | `max_turns` 80, `max_wall_minutes`, compaction at 88K tokens. | — | Already covered. |
| **bg-shell** (`ShellStart` with `wake_on`: exit, match, silence) and **ShellSession** (stateful cwd). | `run` is stateless with a 120 s default (600 s max) timeout; the model uses `cd x && …` and `nohup … &`. | No qh Terminal-Bench traces yet. | Revisit after the Tier C baseline (`tbC-base`): is time lost to polling or timeouts? |
| **Output hygiene** in the TB adapter: strip ANSI, keep the last `\r` segment, collapse duplicate lines. | `run` returns raw output; cap + spill. | Over the 30 TB oracle logs, normalising saves only ~5% (696K → 660K chars). | Not worth a change. |
| **Plan mode → fresh implement session.** | Compaction with a model checkpoint plus git state. | b05 timed out after a 20-min runaway, not from context growth. | Revisit if Tier B failures turn out to be context drift. |
| **Workspace discovery**: read `.docs`, `SPEC.md` etc. first. | System prompt "Orient" step | Traces read SPEC.md and the stubs on turn 1. | Already covered. |
| **Project AGENTS.md** injected once at the tail (4000 chars max). | Task prompt only | Our tasks have no AGENTS.md. | Not applicable to the eval. |

## Candidate L1: `think_cap_continue` (little-coder's thinking-budget, adapted)

The two config knobs are `think_budget_tokens: 8000` and `think_cap_continue: true`, in the candidate harness
`harness/cand/l1-think-cap`.

When a turn's reasoning passes 8K tokens before any content or tool call, the stream is cut. Then:
1. The partial reasoning is kept, as an assistant message whose `reasoning_content` holds it.
2. A short harness user message says "act now; don't draft code in your reasoning".
3. The next request runs with `enable_thinking:false`. Toggling thinking is cache-neutral on this server
   (ENDPOINT.md). Thinking is back on from the turn after.

This differs from EXP-3 in two ways: the partial reasoning is not thrown away, and the cap is 8K instead of 3K.
At 8K the cap sits above most design turns: only 9 of 1429 turns went past 8K.

The rule: adopt if the capped tasks finish faster without losing score, and the cap never fires on Tier-A-style
tasks. Probe: `eval/results/expL1-think-cap` on b01, b04, b05, h03 and a10 ×1, against `it0-baseline`.
Runaways are stochastic, so read the per-firing traces too: did the act turn make a tool call, and how many
turns and minutes did it take to recover?
