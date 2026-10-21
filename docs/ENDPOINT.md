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
- A shared system prompt is reused across *different* conversations (cross-task cache sharing works), so the
  harness keeps the system message identical across tasks and sub-agents.
- History `reasoning_content` is rendered back into the prompt by default (`preserve_thinking`): a 280-token thought
  added 281 prompt tokens. Keeping it verbatim keeps the prefix stable; `preserve_thinking:false` strips it.

## Cloudflare constraints
- Quick tunnels cut responses that send nothing for ~100 s → the client always **streams**.
- Transient 502s happen while the origin restarts → the client retries with backoff for up to 45 minutes.
