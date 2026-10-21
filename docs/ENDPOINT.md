# The endpoint: measured behaviour

All numbers measured from the cloud container on 2026-10-21 against `$REMOTE_LLM_ENDPOINT`.
They are inputs to the harness design, so re-measure if the serving setup changes.

## Identity
- Server: **splash** (MLX runtime; `/metrics` exposes `splash_*` Prometheus metrics, `/health` returns `{"status":"ok"}`).
- Model: `mlx-community/Qwen3.8-27B-4bit`, `max_model_len` = 262,144, vision-capable.
- Exposed through a **Cloudflare quick tunnel** (`*.trycloudflare.com`). The URL changes when the tunnel restarts;
  the server must allow the tunnel host (`--allowed-host`) or it answers 403.
- No `/tokenize`, `/props` or `/docs` endpoints.

## API surface
- OpenAI `POST /v1/chat/completions`, streaming (`stream: true`, `stream_options.include_usage`) and non-streaming.
- Native `tool_calls` (structured, OpenAI deltas when streaming). Smoke test: 49/50 single-tool prompts produced a
  correct structured call at `reasoning_effort=low` (the one "miss" was a sensible read-before-edit).
- `reasoning_effort` (`low`/`medium`/`xhigh`) is honoured; reasoning arrives in `reasoning_content`.
- `chat_template_kwargs` is honoured (`preserve_thinking`, `enable_thinking`).
- Usage includes `prompt_tokens_details.cached_tokens` and `completion_tokens_details.reasoning_tokens`;
  a `metrics` block adds `prefill.tokens`, `cache.status` (`hit`/`miss`) and `cache.matched_tokens`.

## Speed (single request)
| | measured |
|---|---|
| cold prefill | ~80–140 tok/s (2.8K tokens: 21 s; 7.2K tokens: 51 s) |
| decode | ~30–53 tok/s |
| TTFT on a cache hit with ~50 new tokens | ~0.6 s |

Prefill is **10× slower** than the llama.cpp numbers in BUILD_PLAN.md (~1,270 tok/s). A cold 100K-token prompt would
take ~12–20 minutes. Cache discipline is therefore the single most important performance property of the harness.

## Prefix cache
- Prompt state is cached at the end of every request (rounded down to a 32-token page) and at coarse
  checkpoints (~4,096-token granularity) inside the prompt.
- A strict append-only extension reuses everything up to the previous prompt's end (`matched_tokens` ≈ previous
  `prompt_tokens`).
- **Generated tokens are not cached**: the assistant message is re-prefilled on the next turn. Long `write_file`
  calls therefore cost their length again in prefill (≈1K tokens ≈ 8 s).
- A divergence in the middle of the prompt falls back to the last ~4K checkpoint before it
  (test: divergence at ~6.4K → 4,096 matched).
- A shared prefix is reused across *different* conversations only if a cached checkpoint lies inside it.
  In the probe the whole first request was the shared system prompt (so its end-checkpoint was reusable);
  in real runs the ~2K-token system+tools prefix is shorter than the ~4K checkpoint interval and the first
  turn of each task is a full miss (a03 T1: cached=0). The harness still keeps the system message identical
  across tasks and sub-agents so a prefix warm-up (idea, not implemented) could make it reusable.
- History `reasoning_content` is rendered back into the prompt by default (`preserve_thinking`): a 280-token thought
  added 281 prompt tokens. Keeping it verbatim keeps the prefix stable; `preserve_thinking:false` strips it.

## Cloudflare constraints
- Quick tunnels cut responses that send nothing for ~100 s → the client always **streams**.
- Transient 502s happen while the origin restarts → the client retries with backoff for up to 45 minutes.

## Concurrency (measured from it0-baseline traces, 2 concurrent agents)
- Decode per stream: median 14 tok/s (p10 6.7, p90 19.1) → ~28 tok/s total, i.e. no gain over one stream.
- Prefill under load: median 44 tok/s (vs 80–140 alone).
- Conclusion: the server is effectively single-throughput. Run evals with `--parallel 1`: same total
  throughput, cleaner per-task latency. Compare wall time only between runs made at the same concurrency;
  compare success/turns/tokens across any runs.

## reasoning_effort is part of the cached prefix
The template renders the effort near the start of the prompt (same messages: low=1921, medium=1895,
xhigh=1934 prompt tokens), so requests at different efforts share no cache:
`low` then `medium` → `cached=0`; `medium` again → `cached=1888`; `xhigh` → `cached=0`.
Rules for the harness: keep one effort per context segment (a planner/executor effort split costs a full
re-prefill), and make side requests (compaction checkpoint, warm-up) use the segment's effort and tools.

## Prefix warm-up
A 1-token request whose prompt is system + tools + a trivial user turn leaves a checkpoint right after the
shared prefix: the next conversation with a different user message reused 1,888 of 1,952 tokens
(2.9 s vs 43.5 s cold, under load). `qh` does this at the start of every agent (main, sub-agent).
