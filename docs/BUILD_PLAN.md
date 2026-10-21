# Harness² × Qwen3.8-27B: How Google's Harness² Works and a Build Plan for a 128K-Context Local Agent Harness

**Bottom line: Harness² (google-research/harness2) is not an agent runtime you can run Qwen inside. It is a research framework that automatically rewrites the harness *around* an existing agent (OpenCode or Codex), using a strong "editor" model. The right move is to build your own small, Go-based execution harness whose on-disk layout copies Harness²'s eight-component design, run Qwen3.8-27B inside it under a strict prefix-stable context policy, and use Claude Opus as the Harness²-style editor (Propose → Probe → Compose) until Qwen can take over most of that role.**

## TL;DR

- **What Harness² is:** an Apache-2.0 Python research release (paper arXiv:2610.14224, Oct 9, 2026, Google Cloud AI Research + Northwestern). It keeps the model frozen and evolves eight file-based harness components per task through a contrastive Propose–Probe–Compose loop, plus a library of adopted harnesses and lessons that carries across tasks. Its documented path is Vertex AI + OpenCode/Codex. Local endpoints are only clearly supported through the Codex substrate (any Responses API endpoint) and the JobBench judge, so **reimplement its patterns rather than adopt its code as your runtime**.
- **What Qwen3.8-27B is:** a real, dense 27B hybrid model (48 Gated-DeltaNet + 16 full-attention layers), released Aug 2026 under Apache 2.0. It has 262,144-token native context, thinking on by default (`reasoning_effort` xhigh/medium/low), Qwen3-Coder-style XML tool calls (`qwen3_coder` parser), and strong self-reported agentic scores (Terminal-Bench 2.1 73.0, SWE-bench Pro 61.7). The "Qwen3.8-27b" name in your request matches exactly; no substitution was needed.
- **The design constraint that matters most is cache stability, not raw context size.** On llama.cpp the hybrid architecture can only restore a few checkpoints near the end of the prompt, so any edit inside earlier context forces a full re-prefill. Build an append-only, prefix-stable loop with rare, deliberate compaction, file-based memory, capped tool output, and fresh-context sub-agents. Then let Opus improve the harness files against a fixed eval suite (Terminal-Bench 2.1 subset + Go tasks + the caça-palavras PWA), with stopping criteria and a planned hand-off of the editor role to Qwen.

---

## Part 1 — What "harness2" is and how it works

### 1.1 Identification (unambiguous)

"harness2" is **Harness²: Recursive Agent Harnessing for an Open World**, repo `github.com/google-research/harness2`. The arXiv paper (2610.14224v1) was submitted 2026-10-09 by Ruiyao Xu, Yanfei Chen, Zhongying CuiZhu, Bhavana Dalvi Mishra, Yifei Ming, Han Yu, Rujun Han, Chen-Yu Lee and Tomas Pfister (Northwestern University and Google Cloud AI Research).\[1\]\[2\] The README states it is "not an officially supported Google product" and is not eligible for Google's OSS vulnerability rewards program.\[3\] It is unrelated to "Harness" the CI/CD company (harness/harness), which is the main name collision.\[4\] Star counts reported for the repo range from about 2,600 after ten days and about 6,600 after two weeks (secondary sources) to 13.1k stars and 1.2k forks on the GitHub repo page itself, with Trendshift logging +12,872 stars in the 7 days to Oct 20, 2026, so treat popularity figures as fast-moving and noisy.

**Packaging and maturity:** `pyproject.toml` declares `harness2` version **0.1.0**, Apache-2.0, `requires-python >=3.12,<3.14`. CLI entry points are `harness2`, `harness2-judge`, `harness2-report`, `harness2-setup`, `harness2-check`, `harness2-demo` and `harness2-bridge`; the optional `bridge` extra pins LiteLLM proxy and FastAPI.\[5\] Setup docs target Linux + Python 3.12 + uv + Node.js 20+ + the Google Cloud CLI, with Docker for WorkBuddy-Bench.\[6\]\[7\] Benchmark setup clones a pinned OpenCode checkout plus the upstream benchmark and applies patches.\[6\] It is clearly early research code. An independent walkthrough at commit 1cfe254e found the README's `cp .env.example .env` step fails because the file is missing.\[6\] There is no spend cap yet (issue #4 proposes `--max-usd`), and an open proposal (#5) asks for `--bench local` to run on your own folder of tasks.\[6\]\[8\]\[9\]

### 1.2 Core concept: execution harness vs. improvement harness

Harness² treats an agent as **Agent = frozen Model + Harness**.\[10\] It separates two harnesses:\[2\]

- **Execution harness:** what the executor model runs with. Following the AHE file-level decomposition, it exposes **K = 8 editable components, each mounted as a file**:\[2\]\[3\]

| # | Component | File (OpenCode) | When it reaches the model |
|---|---|---|---|
| C1 | System prompt | `systemprompt.md` | Every call |
| C2 | Guardrails | `guardrails.md` | Every call |
| C3 | Memory | `LongTermMEMORY.md` | Every call (durable domain facts) |
| C4 | Tool guidance | `tool_descriptions/tool_guidance.md` | Every call |
| C5 | Subagents | `sub_agents/<name>/AGENT.md` | Via task tool; **own context window, returns only its result** |
| C6 | Skills | `skills/<name>/SKILL.md` (+ scripts) | Loaded only when invoked |
| C7 | Scripts | `scripts/<name>.py` | Run via bash: "a new deterministic capability, executed instead of being reasoned through token by token" |
| C8 | Plugin | `plugin/<name>.ts` | Hook inside the tool loop; "enforces a behavior rather than instructs it" |

- **Improvement harness:** the editor/composer/selector/reflector prompts plus a **harness improvement library**. In the paper the editor and selector are Claude Opus 4.8 at high reasoning effort, running as agentic OpenCode sessions.\[2\]\[6\]

### 1.3 The algorithm

**Task-level recursion (one contrastive step = Propose → Probe → Compose):**

1. **Reference run:** execute the current harness on the task.\[6\] This "reveals how the harness behaves, not whether its deliverable is correct."\[2\]
2. **Propose two scoped candidates:**\[2\]
   - an **invariant (INV)** candidate targeting cross-domain agentic weaknesses (planning, tool use, context management, delegation);\[2\]\[6\]
   - a **domain (DOM)** candidate targeting the domain's knowledge, procedures and constraints.\[2\]\[6\]
   Each candidate is an edited copy of the component files.\[2\]
3. **Probe:** run each candidate on the same task under matched conditions and compare trajectories and deliverables with the reference. This produces a **behavioral contrast**, not a correctness verdict.\[2\]\[3\]
4. **Compose:** read the reference, the candidates, their diffs and the evidence, then merge supported edits, revise conflicts, adopt one candidate, or keep the current harness.\[6\] "Each decision is tied to a concrete observation rather than a presumed score gain."\[2\]

The budget `k` scales this. **Parallel** mode probes k INV + k DOM candidates, then runs k independent compositions over the full pool. **Sequential** mode chains k steps, carrying the refined harness and evidence forward.\[11\] For k>1, an agentic selector compares anonymized deliverables and returns one existing output.\[2\] Cost per task per mode is **4k executor rollouts and 3k improver sessions**, plus selection and reflection, as documented by the repo and verified against the demo by an independent tester.\[6\]\[12\] The paper capped each rollout at 200 model turns and 3,600 s.\[2\]\[6\]

**Harnessing-level recursion (across tasks):** after each task, a reflector reviews the whole improvement record. It stores the adopted harness tree plus an `EXPERIENCE.md` (applicable situation, lessons, supporting evidence) in a per-domain library with an index.\[6\]\[11\] "Lessons are retained only when supported by execution contrasts." Later tasks start from the common base harness but can read, copy and re-verify earlier components.\[6\] Earlier components are not inherited automatically.\[2\]

**Guards against reward hacking:** no improver session ever sees a rubric, score or grader verdict.\[6\] A guard refuses to hand over any working directory containing one. Evidence is passed as files, not inlined in prompts. Invalid trees are retried once with the validator message, then fall back to the starting tree.\[2\]\[6\] Validation includes:
- a skill must be named verbatim in always-on text;\[2\]
- no Jinja template braces in always-on text;\[2\]
- every referenced script must ship with the harness;\[2\]
- OpenCode always-on text is capped at 24,000 characters.\[6\]

### 1.4 What the evidence says

- One step (k=1) improved every configuration. Gains reached up to **+11.5 on WorkBuddy-Bench** (GLM-5/OpenCode, 64.2→75.7), **+4.5 on JobBench** and **+11.8 pp LAB Partial**.\[3\]\[13\] Parallel recursion at k=5 added a further 2.9–3.9 (WorkBuddy), 3.0–4.2 (JobBench) and 2.6–6.7 pp (LAB).\[2\]\[14\]\[15\]
- **Weaker executors gained more.** GLM-5 with an improved harness beat base Gemini 3.5 Flash on WorkBuddy-Bench.\[2\]\[15\] This matters directly for a 27B local model.
- **The editor does not need to be stronger than the executor.** With the same model as executor and editor (OpenCode, k=2), all five models from GLM-5 to Claude Opus 5 improved on every benchmark;\[6\] GLM-5 gained 13.7 on LAB.\[2\]\[6\] This is the basis for eventually replacing Opus with Qwen as editor.
- **Harnesses transfer across models.** A harness refined for Gemini 3.5 Flash raised GLM-5 by 17.3 on WB-Code (58.5→75.8) with no further adaptation.\[2\]
- **Ablations (k=1):** removing the library cost 3.1/1.8, removing scoping cost 2.9/3.7, and removing composition cost 1.6/1.9 (WorkBuddy/JobBench).\[6\] Removing skills from an improved harness cost the most on Code/Web; removing scripts cost the most on Office (7.4).\[2\]
- **Tool reliability:** one step cut tool-error rates on all three benchmarks, e.g. JobBench 14.4%→9.1%.\[2\]
- **Concrete edit patterns from the case studies:**
  - forbid pseudo tool calls emitted as plain text (WB-Code 60→100);\[2\]
  - force an input inventory plus a verification script that reopens produced files (JobBench 43.1→77.8);\[2\]\[6\]
  - a plugin that appends a warning after N tool calls without an artifact.\[2\]
- **Failure modes (paper's own analysis):** the selector picking a weaker candidate (55 chosen over 95), incorrect premises propagating across candidates, and outcome variance.\[2\]

### 1.5 Model-agnostic hooks and local feasibility

- **Executors (substrates):** OpenCode (default) and Codex (`--substrate cx`).\[3\]\[6\]\[15\]\[16\] An issue thread (#64) describes the substrate interface as `surface`, `materialise`, `execute`, `read_trajectory` and `preflight` under `harness2/substrate/`.\[17\] That makes a new substrate (e.g. a Go agent behind a CLI) a bounded piece of work. This last point is my inference, not documented.
- **Benchmark adapter protocol:** the core is written against `BenchmarkAdapter`, whose docstring says "Implement these and the whole method works."\[6\] It requires:\[9\]
  - `tasks()`, `domain_of()`, `task_prompt()` and `task_files()`;\[9\]
  - `execute(task_id, harness_dir, run_id)`;\[6\]
  - `delivered()`, `trajectory()` and `deliverable_text()`;\[6\]
  - `base_harness()`, `validate_tree()` and `score()`.
- **Local endpoints — verified:**
  - `HARNESS2_SOLVER_MODEL` uses OpenCode's `provider/model` format.\[6\]\[7\]
  - Codex accepts "another Responses API endpoint" via `HARNESS2_CODEX_BASE_URL`/`_API_KEY`/`_MODEL`.\[7\]
  - The JobBench judge accepts `HARNESS2_JUDGE_BASE_URL`.\[18\]
  - WorkBuddy setup refuses anything but `google-vertex/gemini...` IDs.\[6\]\[18\]
  - `harness2-check` calls `gcloud auth application-default print-access-token`.\[6\]\[7\]
- **Local endpoints — not verified:** whether the OpenCode substrate preserves a custom `@ai-sdk/openai-compatible` provider block with a local `baseURL` (OpenCode itself supports this for llama-server).\[19\]\[20\] Read `harness2/substrate/opencode.py` before relying on it.
- **Bottom line:** running Harness² *unchanged* fully locally is not a documented path today.\[21\]
- **Safety:** executor network access is unrestricted and Codex runs with its sandbox disabled.\[3\] An independent tester reports a validation rollout that downloaded its own JobBench rubric. Run anything Harness²-derived in a disposable container with egress control.\[6\]

### 1.6 What does *not* exist in Harness²

Harness² does not implement its own agent loop, planner, compaction, retrieval or state persistence. Those come from the wrapped executor (OpenCode/Codex). What it *does* provide is a disciplined, evidence-gated method for deciding **which** prompt, skill, script, sub-agent or plugin changes to make, plus a component taxonomy that prices each lever by its context cost.

### 1.7 Ideas especially valuable for a 27B model in 128K

1. **Price every lever by its context cost.** The editor prompt says always-on prose is "the expensive option… every word of it is in context on every run." It prefers, in order, skill → script → subagent → plugin → always-on text, and "if you add always-on text, cut something else to pay for it."\[2\]
2. **Subagents only for "large input, small answer"** sub-tasks. These are the only component that adds a fresh inference context.\[2\]
3. **Plugins for mechanical context hygiene:** "trimming noisy or oversized tool output so it stops crowding context; appending a targeted hint after an error signature that keeps recurring; capping or steering a tool that runs away; and verifying a write immediately after it happens."\[2\]
4. **Scripts replace token-by-token reasoning** for deterministic operations.\[2\] This is the cheapest reliability win for a smaller model.
5. **Write verified facts, not procedures.** Encode "in this workspace, X maps to Y" rather than instructions for rediscovering it.\[2\]
6. **Every edit must trace to a trajectory moment.** Prefer divergences across runs (the model can already do it), then repeated failures, then visible risks.\[2\]
7. **Contrastive acceptance without ground truth,** plus a reusable, per-domain library of executable harness pieces.

---

## Part 2 — Qwen3.8-27B: verified details

### 2.1 Identity and architecture

The official Hugging Face card is `Qwen/Qwen3.8-27B` (Apache-2.0, architecture tag `qwen3_5`, "Image-Text-to-Text").\[22\] NVIDIA's NGC listing and Artificial Analysis give a release date of **Aug 14, 2026**;\[23\]\[24\] locallyuncensored.com instead dates the Apache 2.0 weight drop to August 13, 2026. The card's specification:

- **Dense** causal LM with vision encoder, 27B parameters (HF shows "28B params" BF16 including vision).\[22\]
- 64 layers, hidden 5,120, FFN 17,408, vocab 248,320 (padded).\[22\]\[25\]
- Layout: **16 × (3 × (Gated DeltaNet → FFN) → 1 × (Gated Attention → FFN))**.\[22\]\[26\]\[27\]
  - Gated DeltaNet: 48 V / 16 QK linear-attention heads, head dim 128.\[22\]
  - Gated Attention: 24 Q / 4 KV heads, head dim 256, RoPE dim 64.\[22\]
- **MTP head** trained with multiple steps,\[22\] usable as a built-in speculative drafter.\[26\]
- **Context:** 262,144 native, "extensible up to 1,000,000 tokens" via static YaRN.\[22\]\[28\]\[29\] Unsloth writes 1,010,000; minor discrepancy.\[26\]\[30\] The card warns static YaRN can hurt shorter texts, so **do not enable YaRN for a 128K deployment**.\[22\]

### 2.2 Modes, template and sampling

- **Thinking is on by default** (`<think>…</think>`). Per request, `chat_template_kwargs.enable_thinking=false` disables it.\[22\]\[25\]\[26\]
- **`reasoning_effort`:** `xhigh` (default), `medium`, `low`.\[22\]\[25\]\[30\]
- **`preserve_thinking`** is on by default and retains thinking from all historical messages. Qwen says this "improves KV cache utilization."\[22\]
- **Recommended sampling:**\[30\]
  - Thinking: `temperature=1.0, top_p=0.95, top_k=20, min_p=0, presence_penalty=0, repetition_penalty=1.0`.\[22\]\[30\]
  - Non-thinking: `temperature=0.7, top_p=0.80, top_k=20, min_p=0, presence_penalty=1.5`.\[22\]\[25\]\[30\]
  - `presence_penalty` may be raised to 2 against repetition, at some risk of language mixing.\[22\]
- **Card caveat that matters for harness design:** "In multi-turn agentic tasks, lower reasoning effort does not always reduce overall task completion time… it can also lead to insufficient analysis, more failures, and repeated retries."\[22\]\[25\]
- **Tool-call format:** the chat template asks the model for Qwen3-Coder-style XML: `<tool_call><function=name><parameter=k>v</parameter></function></tool_call>`.\[31\] vLLM and SGLang decode it with `--tool-call-parser qwen3_coder`. SGLang's cookbook warns that the Hermes parser reads a *different* payload (bare JSON), so tool calls "never parse" if mismatched.\[32\]

### 2.3 Benchmarks and independent signal

| Benchmark (Qwen-reported) | Qwen3.8-27B | Qwen3.6-27B | Opus 4.6 Max |
|---|---|---|---|
| Terminal-Bench 2.1 (Terminus) | 73.0 | 63.4 | 78.2 |
| SWE-bench Pro | 61.7 | 53.5 | 53.4 |
| NL2Repo-Bench | 42.3 | 36.2 | 47.6 |
| DeepSWE 1.1 | 42.2 | 13.3 | — |
| IFBench | 79.5 | 69.1 | 62.5 |
| LiveCodeBench v6 | 90.3 | 83.9 | 88.8 |

Read these with three caveats:

- Most coding numbers were produced **with the Claude Code harness, temp 1.0, and a 256K context window**, i.e. twice your ceiling. Several benchmarks are in-house (QwenSWEBench, CoWorkBench).\[22\]
- **Artificial Analysis** independently scores the xhigh variant at 34 on its Intelligence Index (v4.3.2). It generated 200M tokens during the evaluation against an 82M median ("very verbose") and ran at 45 tokens per second against an average of 79 ("slower than average"). OpenRouter's mirror lists:

  | AA index | xhigh | medium | low |
  |---|---|---|---|
  | Intelligence | 33.7 | 27.6 | 26.2 |
  | Coding | 68.1 | 56.1 | 58.2 |
  | Agentic | 45.8 | 44.4 | 38.9 |

  Linas's Newsletter ("The Ultimate Guide to Qwen3.8-27B") and VentureBeat both claim "52" on the same index. That conflicts with the AA page and should be disregarded.
- **Simon Willison** found xhigh "spectacular over-thinking":\[33\] a single SVG took 21 minutes and 22,276 reasoning tokens. He got 15–30 tok/s in LM Studio, and the llama.cpp MTP draft mode was about 72% faster. He confirmed it can drive the Pi coding agent.\[33\]

**Interpretation:** the agentic index barely drops from xhigh to medium (45.8→44.4) but drops sharply at low (38.9). **Medium is the right default for executor turns**, with xhigh reserved for planning and debugging turns. Ablate this; don't assume it.

### 2.4 Memory math for your 128K ceiling

Only the 16 full-attention layers hold a growing KV cache.\[27\] At fp16 that is 2 × 4 KV heads × 256 dim × 2 bytes × 16 layers = **64 KB/token**, so **128K ≈ 8 GB**; the 48 DeltaNet layers hold a constant recurrent state.\[25\]\[34\] Representative measured footprints:

- Q4_K_M weights are **17.1 GB** (Unsloth repo), so 128K at f16 totals about 25.1 GB.\[25\]
- On vramcalculator.com's 24 GB L4 (which reports 22.5 GB usable), 128K with an F16 cache **ran out of memory while loading**. `-ctk q8_0 -ctv q8_0` brings 128K down to **20.21 GB** (21.14 GB with vision), with a measured peak of 19.67 GB on the card (UD-Q4_K_M).

That combination (24 GB GPU, Q4 weights, q8_0 KV) is almost certainly why your ceiling is 128K. The rest of this plan assumes it.

### 2.5 Serving options and how they interact with 128K

**llama.cpp / llama-server (recommended for a single 24 GB consumer GPU)**

- Strengths:
  - mature GGUF quants (Unsloth UD-Q4_K_XL is the documented default);
  - KV quantization (`--cache-type-k/v q8_0`);
  - MTP self-speculation (`--spec-type draft-mtp`);
  - `--jinja` chat templates;
  - reasoning effort via `--chat-template-kwargs '{"reasoning_effort":"medium"}'`;\[30\]
  - GBNF/JSON-schema constrained output.
- **Critical caveat for hybrid models:** llama-server cannot roll recurrent state back to an arbitrary position. A detailed qwen-code bug report (Qwen3.8-27B Q4_K_M, 116,736 context, q8_0 KV) shows it restores only saved checkpoints placed about **5 and 513 tokens before the end** of each prompt. When a request diverged 717 tokens before the previous end, it logged "forcing full prompt re-processing" and re-prefilled 24,782 tokens in 19.5 s (≈1,274 tok/s). When each request was a strict extension, it processed only the 2,487 new tokens in 2.2 s. Turn time fell from 108 s to 34 s just by keeping the prefix stable.\[35\]
- Checkpoints cost about **150 MiB each** without MTP but about **890 MiB with MTP** (llama.cpp issue #30502).\[36\] Keep `--ctx-checkpoints` small when MTP is on, or budget host RAM.
- Tool-call parsing for Qwen3-Coder-style XML has a history of template and parser bugs in llama.cpp, e.g. issue #30078: well-formed `<tool_call>` blocks returned as plain content with `finish_reason: stop`.\[37\] The harness must tolerate and repair this (see 3.6).
- The vcruz305/Qwen3.8-27B-GGUF model card reports that the official jinja template wraps every assistant turn in think tags even when reasoning is empty and then opens another `<think>` on generate, which "truncates multi-turn agents"; that pack (and others) ship a replaced template. Pin your template.

**vLLM (best if you have Blackwell or more VRAM)**

- The official recipe uses `--reasoning-parser qwen3 --enable-auto-tool-choice --tool-call-parser qwen3_coder`, NVFP4/FP8/MXFP4 variants, `--kv-cache-dtype fp8`, and MTP via `--speculative-config '{"method":"mtp","num_speculative_tokens":3}'`.\[26\]
- The recipe stresses that `--reasoning-parser qwen3` "is not optional in practice."\[26\]
- On **1× RTX 5090**, NVFP4 only fits with `--enforce-eager` and was shown at `--max-model-len 32768`.\[26\] That is a poor fit for 128K on 32 GB unless you accept tight KV pools.
- Prefix caching for GDN hybrids runs in experimental **`align`** mode.\[38\]\[39\]\[40\]\[41\] Open bugs report NaN logits after certain prefix-cache hits (vLLM #55766, v0.28.0) and hit rates silently dropping to 0% when the align checkpoint lands in request-unique tokens (#45238).\[39\]\[42\]
- vLLM #61206 reports three `qwen3_coder` parser bugs, including a literal `</think>` vanishing from tool arguments.\[43\]

**SGLang** uses the same parsers (`--reasoning-parser qwen3 --tool-call-parser qwen3_coder`) and exposes an Anthropic-compatible endpoint. Unsloth notes SGLang needs v0.5.19 for its NVFP4 build.\[30\]\[32\] **Ollama / LM Studio / MLX** are convenient; Ollama runs llama-server internally, so the checkpoint caveat applies.\[35\] LM Studio's default 8,192 context is far too small for this model.\[33\]

**Recommendation:** use llama-server with Q4 (UD-Q4_K_XL or Q4_K_M), q8_0 KV, `-c 131072`, `--parallel 1`, `--jinja`, MTP on if host RAM allows. Make the harness responsible for prefix stability.

---

## Part 3 — Design: using Harness² to build a 128K local harness

### 3.1 Adopt, fork or reimplement?

| Option | Pros | Cons | Verdict |
|---|---|---|---|
| Adopt Harness² as-is | Proven method, ready prompts | Vertex-first, Python, wraps OpenCode/Codex; 4k rollouts per task is expensive locally; no spend cap; local OpenCode provider unverified | No |
| Fork Harness² + write a substrate/adapter for your Go agent | Reuses editor/composer/library code and leak guards | Couples you to an early research codebase; Python glue; rollout budget still 4k | Later, optional (Phase 6) |
| **Reimplement the patterns in Go** | Fits your stack; full control of the context policy (the real bottleneck); keeps Harness²'s file layout so its prompts and method still apply | You own the loop | **Yes** |

**Recommended architecture:** a Go executor ("qh", for example) that mounts a Harness²-shaped component tree. Opus acts as the improvement harness, following Harness²'s editor/composer prompts (Appendix H of the paper) over trajectories your Go harness writes to disk. Google's ADK for Go is a viable base if you want one:
- its `openaimodel` package (since v2.1.0) targets OpenAI-compatible endpoints;\[44\]
- `session/compaction` (v2.3.0) provides sliding-window and tail-retention compaction with an `LLMSummarizer`.\[45\]

A minimal custom loop on `openai-go` is simpler and gives you exact control over prefix stability, which ADK's summarizer does not guarantee. Start custom; borrow ADK pieces only if they fit.

Keep **OpenCode pointed at llama-server** as a *baseline comparator*. OpenCode documents an `@ai-sdk/openai-compatible` provider with `baseURL http://127.0.0.1:8080/v1`. \[19\]\[20\]\[46\] Every harness change must beat both your previous version and stock OpenCode.

### 3.2 The context policy (the heart of the design)

**Principle: the prompt is an append-only log between compaction events.** Every mutation inside earlier context costs a full re-prefill on llama.cpp hybrids. At the ~1,270 tok/s prefill measured above, re-prefilling 100K tokens costs roughly 80 s; that figure is my extrapolation.

**Budget for `-c 131072` (starting point, tune empirically):**

| Segment | Budget | Rule |
|---|---|---|
| Stable prefix: system prompt + guardrails + memory + tool guidance | ≤ 6K tokens | Harness²'s 24,000-char cap ≈ 6K tokens (approximate, 4 chars/token). Frozen for the whole session. |
| Tool schemas | ≤ 2K | 6–8 tools max |
| Task card (goal, acceptance criteria, current plan step) | ≤ 3K | Injected once per sub-task, never edited in place |
| Working history (append-only) | grows to ~70K | Compaction trigger at **88K total prompt** |
| Reserved for generation (thinking + answer) | 24K | `max_tokens` per turn ≤ 16K; medium effort |
| Safety margin | 8K | Tokenizer variance, tool-output spikes |

**Rules:**

1. **Never edit, reorder or delete earlier messages** between compactions. That includes not stripping old thinking: keep `preserve_thinking=true`, since stripping changes earlier content.
2. **Never inject volatile data at the tail of every request** (timestamps, memory catalogs, "current status" blocks). That is exactly the qwen-code bug.\[35\] Status goes in a *new* appended message only when it changes.
3. **Tool-output cap:** ≤ 4K tokens inline. Anything larger is written to `.qh/spill/<id>.txt` and replaced by a head/tail excerpt + path + line count. The model re-reads ranges on demand with `read_file(path, start, end)`.
4. **Compaction is a planned event, not a reflex.** At 88K, or at a sub-task boundary (preferred, since it's cheaper and cleaner), write a structured checkpoint to `.qh/state/`:
   - `PLAN.md` (step list with status);
   - `PROGRESS.md` (what changed, which files, test status);
   - `DECISIONS.md` (decisions + rationale);
   - `OPEN_ISSUES.md`.
   Then **restart the context** as stable prefix + task card + checkpoint files (~8–12K). Pay one re-prefill and keep going. A deterministic Go summarizer (git diff --stat, test results, plan status) is preferable to an LLM summary. Use an LLM summary only for `DECISIONS.md`.
5. **Sub-agent isolation:** every plan step runs in a **fresh context** whose prefix is the same stable harness prefix, so the server's prompt cache can reuse it. Its input is the task card + relevant files, and it returns a ≤ 1K-token result + files on disk. Use Harness²'s rule: only delegate "large input, small answer" work (code search, log triage, reading a spec section).\[2\]
6. **Retrieval on demand:** prior work lives in files (`.qh/state`, `.qh/spill`, git history). Expose `grep`/`read_file` rather than embedding retrieval at first. Add a vector index only if traces show repeated re-reading failures.

### 3.3 Decomposition and planning within a 27B's reliability limits

- **Two-level plan:** a short milestone plan (3–8 steps) written to `PLAN.md`, each step with a *verifiable* acceptance check (a command that exits 0). The model plans at xhigh effort and executes at medium.
- **Step size:** one step should fit in one fresh context with ≤ 40K tokens of working history. If a step hits compaction twice, the harness forces a re-plan that splits it.
- **Checkpointing:** `git commit` after each step that passes its check, giving cheap rollback. Failure after N=3 attempts reverts to the last checkpoint and escalates (re-plan → Opus).
- **Verification loop:** test-first where possible. The harness, not the model, runs the acceptance command after each "done" claim (a Harness²-style plugin: "verifying a write immediately after it happens"). Claimed-done-but-failing is logged as its own failure class.

### 3.4 Tool design for a small model

Keep **6–8 tools** with flat, string/integer parameters (the XML parameter format handles flat arguments best):

`read_file(path, start_line, end_line)`, `write_file(path, content)`, `edit_file(path, old, new)` (exact-match replace; fail loudly on 0 or >1 matches), `run(cmd, timeout_s)`, `grep(pattern, path_glob)`, `list_dir(path, depth)`, `update_plan(step_id, status, note)` (appends, never rewrites), `delegate(task_card)` (sub-agent).

- Deterministic helpers (formatting, build, test, lint, JSON validation) are **scripts**, not reasoning.
- Every tool result starts with a one-line status (`OK`, `ERROR <class>: <hint>`). Error hints are curated over time; this is exactly the Harness² "plugin appends a targeted hint after a recurring error signature" pattern.

### 3.5 Constrained output

Use grammar/JSON-schema constraints only for **harness-internal structured calls**: plan JSON, checkpoint summaries, reviewer verdicts. Use them via llama-server's JSON schema support, or via a non-thinking request. For normal agent turns, rely on the native XML tool format plus a repair layer, because constraining a thinking model's free-form output tends to fight its template.

### 3.6 Recovery from tool-call format errors and loops

- **Parse repair:** if `tool_calls` is empty but content contains `<tool_call>`/`<function=`, parse it tolerantly (both XML and JSON-argument variants, per llama.cpp #30078) and execute.\[37\] Log `repaired_tool_call`.
- **Pseudo-calls:** if the model describes a call in prose without emitting it, append one corrective user message and retry once. The paper's WB-Code case fixed exactly this with tool guidance.\[2\]
- **Loop detection:** hash (tool, args). If the same call appears 3× in 8 turns, or there are N=12 tool calls with no file change and no new test result, append a hint: "you are repeating X; state what you learned and choose a different action." If it persists, force a re-plan.
- **Runaway thinking:** cap per-turn reasoning (llama.cpp reasoning budget / `max_tokens`), drop to medium effort, and log `think_overflow`.

### 3.7 Role of Opus and how to shrink its usage

| Phase | Opus role | Qwen role |
|---|---|---|
| Build (Phases 0–3) | Writes the Go harness, the eval runner and the initial component files | Executor under test |
| Improve (Phases 4–5) | **Harness editor/composer** per Harness²: reads failure trajectories (files), proposes INV and DOM edits, composes; never sees hidden test expectations | Executor for probes |
| Hand-off (Phase 6) | Auditor: reviews Qwen-proposed edits weekly | **Editor** for routine edits (supported by the paper's same-model result) |

Opus also acts as an **escalation planner**. When Qwen fails a step 3×, Opus gets the checkpoint files, *not* the transcript, and returns a revised plan. Track Opus tokens per solved task as a first-class metric. Quota levers:
- Opus edits **harness files**, not task outputs;
- the library makes fixes reusable;
- trajectories are summarized by Go scripts before Opus reads them;
- k=1 by default.

### 3.8 Observability

Write a JSONL trace per run: `run_id, task_id, harness_hash, step, turn, prompt_tokens, cached_tokens (or llama-server prompt-eval count), completion_tokens, reasoning_tokens, latency_ms, tool, args_hash, tool_status, error_class, compaction_event, reprefill_tokens`. Also export the llama-server `--metrics` endpoint.

**Failure taxonomy (fixed codes):**
`F1 tool-format`, `F2 wrong tool/args`, `F3 loop`, `F4 think-overflow`, `F5 context-overflow/compaction-loss`, `F6 claimed-done-but-failing`, `F7 spec misunderstanding`, `F8 environment/infra`, `F9 regression of earlier step`, `F10 gave up/empty deliverable`.

The single most important cache metric is **re-prefill tokens per task**.

---

## Part 4 — The plan to hand to Opus (paste everything below this line)

---

### MISSION

You are the lead engineer and **harness editor** for a local coding agent. The executor is **Qwen3.8-27B** served locally (OpenAI-compatible endpoint at `http://127.0.0.1:8080/v1`, hard context limit **131,072 tokens**). Your job:
1. build a Go harness ("qh") that lets Qwen complete complex multi-step coding tasks within that limit;
2. build an evaluation suite;
3. iteratively improve the harness using the Harness² method (Propose → Probe → Compose, evidence-gated, with a lesson library);
4. progressively hand routine harness editing to Qwen itself.

Optimize for task success first, then wall time, then Opus token usage.

### HARD CONSTRAINTS

- C1. Prompt ≤ 131,072 tokens including generation. Target total prompt ≤ 88K before compaction, and reserve ≥ 24K for generation.
- C2. **Prefix stability:** between compaction events, every request must be a strict append-only extension of the previous one. Never edit, reorder or remove earlier messages, never strip earlier thinking, and never append per-request volatile blocks. Verify with server logs (no "forcing full prompt re-processing" except at compaction or sub-agent start).
- C3. Language: Go (single static binary). Python is allowed only for eval glue that external benchmarks require (Harbor).
- C4. Sandbox: all tool execution in a disposable container or workdir with network egress blocked during eval runs.
- C5. You never read hidden test expectations or graders while editing the harness (Harness² leak rule). Edits must cite a specific trajectory moment.
- C6. Change one thing per experiment unless composing already-validated edits.

### KNOWN FACTS TO RELY ON (verify versions on first run)

- Model: dense hybrid (48 Gated DeltaNet + 16 full-attention layers), 262,144 native context (do NOT enable YaRN). Thinking on by default; `reasoning_effort` ∈ {xhigh (default), medium, low}; `preserve_thinking` default true.
- Sampling, thinking: temp 1.0, top_p 0.95, top_k 20, min_p 0, presence_penalty 0. Non-thinking: temp 0.7, top_p 0.8, top_k 20, presence_penalty 1.5.
- Tool calls: Qwen3-Coder XML (`<tool_call><function=…><parameter=…>`). Parsers: vLLM/SGLang `qwen3_coder` + reasoning parser `qwen3`; llama.cpp `--jinja`. Hermes-format parsing will fail.
- llama-server restores hybrid-model state only from checkpoints near the prompt end (≈5 and ≈513 tokens before the end). Any earlier divergence means a full re-prefill.\[35\]
- KV cost ≈ 64 KB/token at f16 (128K ≈ 8 GB); q8_0 halves it.

### STARTING SERVER CONFIG (llama.cpp, 24 GB GPU assumed)

```bash
llama-server \
  -m Qwen3.8-27B-UD-Q4_K_XL.gguf \
  -c 131072 --parallel 1 -ngl 99 -fa on \
  --cache-type-k q8_0 --cache-type-v q8_0 \
  --jinja --metrics \
  --temp 1.0 --top-p 0.95 --top-k 20 --min-p 0.0 \
  --chat-template-kwargs '{"reasoning_effort":"medium","preserve_thinking":true}' \
  --ctx-checkpoints 8 \
  --host 127.0.0.1 --port 8080
# Phase 5 ablation: add MTP  --spec-type draft-mtp --spec-default --spec-draft-n-max 4
# (MTP checkpoints are ~890 MiB each vs ~150 MiB without; watch host RAM)
```

If using vLLM instead: `--max-model-len 131072 --kv-cache-dtype fp8 --reasoning-parser qwen3 --enable-auto-tool-choice --tool-call-parser qwen3_coder --enable-prefix-caching` (hybrid prefix caching is experimental `align` mode; watch for NaN/0%-hit bugs).

### REPOSITORY LAYOUT TO CREATE

```
qh/
  cmd/qh/                 # CLI: qh run --task <dir> --harness <dir> --run-id <id>
  internal/loop/          # agent loop, append-only message log
  internal/ctx/           # token accounting, budgets, compaction, spill
  internal/tools/         # read_file, write_file, edit_file, run, grep, list_dir, update_plan, delegate
  internal/repair/        # tool-call parse repair, loop detection
  internal/trace/         # JSONL traces + summaries for the editor
  harness/base/           # Harness²-shaped component tree (the thing you will edit)
    systemprompt.md  guardrails.md  LongTermMEMORY.md
    tool_descriptions/tool_guidance.md
    skills/<name>/SKILL.md (+scripts/)   scripts/   sub_agents/<name>/AGENT.md
    hooks/                # Go-side "plugins": output trimming, error hints, write verification
  library/<domain>/<task>/{harness/, EXPERIENCE.md}  + INDEX.md
  eval/{tasks/, runner/, results/}
```

Rules carried over from Harness² validation:
- every skill directory name must appear verbatim in `systemprompt.md` or `guardrails.md` as a trigger line;
- every referenced script must exist;
- always-on text total ≤ 24,000 characters.

### PHASED ROADMAP

**Phase 0 — Baseline and instrumentation (day 1–2)**
- [ ] Start the server. Run 10 smoke prompts. Confirm structured `tool_calls` and separated reasoning are returned.
- [ ] Configure stock OpenCode against the same endpoint as a comparator.
- [ ] Write the trace schema and a `qh report` that prints per-task metrics.
- Acceptance: tool calls parse in ≥ 95% of 50 scripted single-tool prompts; trace captures prompt/completion/reasoning tokens and re-prefill events.

**Phase 1 — Minimal Go loop (day 3–6)**
- [ ] Append-only loop with the 8 tools, per-turn `max_tokens` 16K, medium effort.
- [ ] Tool-output cap 4K tokens + spill files.
- [ ] Parse-repair layer and loop detector (3 identical calls in 8 turns).
- Acceptance: on the Tier-A tasks (below), success ≥ stock OpenCode; zero unexpected re-prefills (C2) in 20 runs.

**Phase 2 — Context management (week 2)**
- [ ] Token accountant using the model tokenizer (or the server `/tokenize` endpoint).
- [ ] Compaction at 88K or at sub-task boundary: write `.qh/state/{PLAN,PROGRESS,DECISIONS,OPEN_ISSUES}.md`, restart from stable prefix + task card + state.
- [ ] Deterministic progress summary from git/test output; LLM only for DECISIONS.
- Acceptance: Tier-B tasks that exceed 128K of raw history complete without context errors; ≤ 1 re-prefill per compaction; no loss of acceptance criteria after compaction (spot-check 10 traces).

**Phase 3 — Decomposition, sub-agents, verification (week 3)**
- [ ] Planner turn (xhigh) → `PLAN.md` with per-step acceptance commands.
- [ ] Executor steps in fresh contexts (medium); `delegate` for large-input/small-answer reads.
- [ ] Harness-run acceptance checks after every "done"; git commit per passed step; revert + re-plan after 3 failures; escalate to Opus after 2 re-plans.
- Acceptance: Tier-B success +15 points over Phase 2; F6 (claimed-done-but-failing) < 5% of steps.

**Phase 4 — Harness² improvement loop with Opus as editor (weeks 4–6)**
- [ ] For each failing task cluster, run Propose → Probe → Compose at k=1. For each failed task:
  1. Gather 2–3 reference runs.
  2. Write one **INV** candidate (procedure/tools/context) and one **DOM** candidate (Go/web/PWA knowledge as skills).
  3. Probe each once.
  4. Compose only edits supported by a named trajectory moment.
- [ ] Prefer levers in this order: skill → script → sub-agent → Go hook → always-on text. If always-on text grows, cut elsewhere.
- [ ] After adoption, write `library/<domain>/<task>/EXPERIENCE.md` (situation, lesson, evidence, confidence). Keep only contrast-supported lessons.
- Acceptance: each adopted edit improves its cluster on held-out tasks (not the tasks used to derive it) by ≥ 1 task, with no regression > 1 task elsewhere.

**Phase 5 — Optimization ablations (week 7)**
- [ ] Reasoning effort per role (planner xhigh / executor medium vs. all medium vs. executor low).
- [ ] MTP on/off; compaction threshold 80K/88K/100K; tool cap 2K/4K/8K; `preserve_thinking` (expect off to break cache — measure it).
- Acceptance: choose the config on the Pareto frontier of success × wall time; document the numbers.

**Phase 6 — Hand-off of the editor role (week 8+)**
- [ ] Let Qwen run the same editor prompts on new failure clusters. Opus reviews diffs only.
- [ ] Optionally write a Harness² `BenchmarkAdapter` + Go-agent substrate to run the upstream pipeline locally (fork; read `harness2/substrate/opencode.py` first).
- Acceptance: Qwen-proposed edits accepted by the same held-out test at ≥ 50% the rate of Opus edits; Opus tokens per solved task down ≥ 70% vs. Phase 4.

### EVALUATION SUITE

- **Tier A (smoke, 15 tasks, < 15 min each):** small Go tasks with `go test` acceptance. Examples: add an HTTP handler + test, fix a race reported by `-race`, refactor a package without changing behaviour, write a CLI flag parser.
- **Tier B (multi-step, 10 tasks, 30–120 min each):** repo-scale tasks that force > 128K of raw interaction. Examples: implement a feature across 5+ files, migrate a module's API, debug a failing integration test from logs.
- **Tier C (external):** a fixed 20-task subset of **Terminal-Bench 2.1** run via Harbor with a custom agent (`harbor run -d terminal-bench/terminal-bench-2-1 --agent-import-path …`).\[47\]\[48\] Pick tasks that run locally in Docker without GPUs. Keep the subset frozen; don't tune on its tests.
- **Tier D (end-to-end product): the caça-palavras PWA.** Use the user's existing written spec, frozen. Hidden acceptance (kept outside the harness workdir):
  - Playwright tests: grid renders at configured size; words placed in all configured directions; selection by drag/tap; found-word state; win state; Portuguese diacritics handled per spec.
  - Lighthouse PWA installability + offline load via service worker.
  - `npm run build` exits 0.
  Score = fraction of checks passed. Run 3 seeds.
- **Protocol:** every configuration × 3 runs (Harness² used 3 shuffled runs).\[2\] Report mean ± SD. Always run current harness, previous harness and stock-OpenCode baseline on the same tasks.

**Metrics per run:**
- success (binary + partial score);
- wall time;
- prompt/completion/reasoning tokens;
- **re-prefill tokens**;
- compactions;
- tool-error rate;
- failure-code histogram (F1–F10);
- Opus tokens spent;
- Qwen turns per solved task.

### IMPROVEMENT LOOP (each iteration)

1. Run the suite. Produce `results/<iter>/summary.md` with failure-code histograms.
2. Pick the top failure cluster by (frequency × severity).
3. Read 3 representative traces **as files** (Go-generated digests: turn list, tool calls, errors, last 40 lines of each failing step). Do not read hidden tests.
4. Write a hypothesis tied to specific turns ("turn 37: model re-read the 9K-token log 4×").
5. Propose INV and DOM candidates (minimal diffs to harness files or Go hooks).
6. Probe on the cluster tasks (k=1). Compose. Validate on held-out tasks.
7. Adopt or reject. Write `EXPERIENCE.md`. Commit with the experiment ID.

### STOPPING CRITERIA

- Stop an iteration line when 3 consecutive adopted edits each yield < 1 task improvement on held-out tasks.
- Stop the project phase when Tier-B success ≥ 70%, Tier-D score ≥ 0.9 on 3/3 seeds, and Tier-C subset ≥ stock-OpenCode baseline + 10 points. These are targets I propose, not sourced benchmarks; revise after Phase 1 data.

### RISKS / OPEN QUESTIONS (check early)

- R1. Tool-call parser bugs (llama.cpp #30078, vLLM #61206).\[43\] Keep the repair layer and count `repaired_tool_call`.
- R2. Chat template defects in some GGUF packs (reported nested think blocks). Pin and checksum the template.
- R3. Hybrid prefix-cache fragility (llama.cpp checkpoints; vLLM align-mode bugs).\[35\]\[36\] C2 is non-negotiable.
- R4. Over-thinking at xhigh.\[33\] Enforce per-turn budgets.
- R5. Benchmarks reported at 256K with the Claude Code harness. Expect lower numbers at 128K; measure your own baseline first.
- R6. Overfitting harness edits to the eval. Always validate on held-out tasks and keep Tier C frozen.
- R7. Harness² code paths for local OpenCode providers are unverified. Phase 6 only.

---

## Caveats

- Many sources here are days to weeks old (Harness² was published Oct 9, 2026; Qwen3.8-27B in Aug 2026).\[1\]\[24\] Repo internals, parser fixes and serving flags are moving quickly; re-verify versions before relying on specific flags.
- Qwen's benchmark table is self-reported, partly in-house, and measured at 256K context with the Claude Code harness. Independent Artificial Analysis numbers are much more modest, and one widely shared guide misquotes them.
- Harness² gains were measured on professional-work benchmarks (WorkBuddy, JobBench, LAB) with frontier and near-frontier executors (GLM-5, Gemini 3.5 Flash), not a local 27B.\[2\]\[6\]\[13\] The transfer to your setup is a reasoned expectation, not a result.
- Not verified: how Harness²'s OpenCode substrate handles a custom local provider. GitHub blocked direct source access; read `harness2/substrate/opencode.py` before attempting Phase 6.
- The budget numbers, thresholds and stopping targets in Part 4 are engineering starting points derived from the cited mechanics, not sourced constants. Phase 5 exists to replace them with measured values.

## Sources

1. [Harness^2: Recursive Agent Harnessing for an Open World](https://arxiv.org/abs/2610.14224)
2. [Harness^2: Recursive Agent Harnessing for an Open World](https://arxiv.org/pdf/2610.14224v1)
3. [GitHub - google-research/harness2 at producthunt · GitHub](https://github.com/google-research/harness2)
4. [github.com](https://github.com/jpascal/harness)
5. [harness2/pyproject.toml at main · google-research/harness2 · GitHub](https://github.com/google-research/harness2/blob/main/pyproject.toml)
6. [Google Harness2 Setup Guide: What It Costs and What to Block](https://www.jahanzaib.ai/blog/google-harness2-setup-guide)
7. [harness2/docs/SETUP.md at main · google-research/harness2](https://github.com/google-research/harness2/blob/main/docs/SETUP.md)
8. [Issues · google-research/harness2](https://github.com/google-research/harness2/issues)
9. [Proposal: \`--bench local\`, run Harness² on your own folder of tasks · Issue #5 · google-research/harness2 · GitHub](https://github.com/google-research/harness2/issues/5)
10. [Harness2: Recursive Agent Harnessing foran Open World](https://arxiv.org/html/2610.14224v1)
11. [harness2/docs/EXPERIMENTS.md at main · google-research/harness2 · GitHub](https://github.com/google-research/harness2/blob/main/docs/EXPERIMENTS.md)
12. [Proposal: \`--max-usd\` spend limit for \`harness2\` runs · Issue #4 · google-research/harness2 · GitHub](https://github.com/google-research/harness2/issues/4)
13. [Google Open-Sources Harness2, a Recursive Agent Harnessing Framework - MarkTechPost](https://www.marktechpost.com/2026/10/18/google-harness2-recursive-agent-harnessing-framework/)
14. [Google Harness²: Setup, Benchmarks and the Real Cost](https://wavect.io/blog/google-harness2-setup-cost-safety/)
15. [Harness² (Google Research) explained: the agent harness that improves itself — Nicolás Torres](https://ntorres.dev/blog/harness2-google-recursive-agent-harness)
16. [Harness2: Google Research Repository Overview (2026)](https://www.activepieces.com/blog/harness2-google-research-repository-overview-2026)
17. [Add substrates for Pi and Claude code. · Issue #64 · google-research/harness2 · GitHub](https://github.com/google-research/harness2/issues/64)
18. [harness2/docs/BENCHMARKS.md at main · google-research/harness2 · GitHub](https://github.com/google-research/harness2/blob/main/docs/BENCHMARKS.md)
19. [Best Open-Source Agent Harnesses for Local LLMs in 2026 - MarkTechPost](https://www.marktechpost.com/2026/09/18/best-open-source-agent-harnesses-for-local-llms-in-2026/)
20. [Providers](https://opencode.ai/docs/providers/)
21. [Harness2: Google's agent that rewrites its own harness — Blog Beer And Code](https://blog.beerandcode.com.br/en/news/harness2-google-agent-rewrites-its-own-harness)
22. [Qwen/Qwen3.8-27B · Hugging Face](https://huggingface.co/Qwen/Qwen3.8-27B)
23. [Qwen3.8-27B](https://catalog.ngc.nvidia.com/orgs/nim/qwen/models/qwen3.8-27b/-)
24. [Qwen3.8 27B (xhigh) - Intelligence, Performance & Price Analysis](https://artificialanalysis.ai/models/qwen3-8-27b)
25. [How to Run Qwen 3.8 27B Locally: VRAM, Quants and the Template Trap](https://locallyuncensored.com/blog/how-to-run-qwen-3-8-27b-locally.html)
26. [Qwen3.8-27B on vLLM — Serve command for GB300 NVL4, B200, RTX 5090, RTX 5090, RTX Pro 6000, DGX Spark (GB10), Ascend 950PR, Arc Pro B60, Arc Pro B70](https://recipes.vllm.ai/Qwen/Qwen3.8-27B)
27. [How to Run Qwen 3.8 Locally: 27B on 16–24GB GPUs (2026 Guide)](https://codersera.com/blog/how-to-run-qwen-3-8-locally-2026/)
28. [Qwen 3.8 27B VRAM Requirements: Runs on 24 / 32 / 48 GB GPUs](https://canitrun.dev/models/qwen3.8-27b/)
29. [Qwen3.8 VRAM: the 27B That Ties GPT-5.6 Fits 24GB](https://vramcalculator.com/qwen3-8-vram-requirements/)
30. <https://unsloth.ai/docs/models/qwen3.8>
31. [froggeric/Qwen-Fixed-Chat-Templates · Hugging Face](https://huggingface.co/froggeric/Qwen-Fixed-Chat-Templates)
32. [Qwen3.8-27B - SGLang Documentation](https://lmsysorg.mintlify.app/cookbook/autoregressive/Qwen/Qwen3.8-27B)
33. [Qwen 3.8 27B is excellent, but it defaults to wildly overthinking things](https://simonwillison.net/2026/Aug/16/qwen-38-27b/)
34. [Run Qwen 3.8 27B Locally: Real GGUF Sizes, the KV Cache Trick, and the Template Trap - DEV Community](https://dev.to/purpledoubled/run-qwen-38-27b-locally-real-gguf-sizes-the-kv-cache-trick-and-the-template-trap-114j)
35. [Request-tail memory catalog (#13521) forces a full prompt re-process on every step with hybrid models on llama.cpp / Ollama · Issue #14203 · QwenLM/qwen-code](https://github.com/QwenLM/qwen-code/issues/14203)
36. [Eval bug: running several sequential sessions with Qwen3.8 is impossible with MTP without insane amounts of host RAM for cache (262k context at 32 checkpoints requires 37 GiB) · Issue #30502 · ggml-org/llama.cpp · GitHub](https://github.com/ggml-org/llama.cpp/issues/30502)
37. [qwen4exp (Qwen3.8-Flash-Next): stochastic tool-call emission returned as content (finish=stop) + nlohmann 302 mid-stream - silent tool-call drops · Issue #30078 · ggml-org/llama.cpp · GitHub](https://github.com/ggml-org/llama.cpp/issues/30078)
38. [Qwen3.5 / Qwen3.6 / Qwen3.8 series](https://docs.lmcache.ai/recipes/qwen3_5.html)
39. [\[Bug\] Qwen3.5/3.8 hybrid GDN: NaN logits after a prefix-cache hit when the previous prefill ended 4-10 tokens past a block boundary (v0.28.0, mamba cache mode align) · Issue #55766 · vllm-project/vllm · GitHub](https://github.com/vllm-project/vllm/issues/55766)
40. [Hybrid Attention Models](https://docs.lmcache.ai/mp/hybrid_models.html)
41. [Whats is Mamba and Hybrid model？ - General - vLLM Forums](https://discuss.vllm.ai/t/whats-is-mamba-and-hybrid-model/2462)
42. [\[Bug\]: Hybrid-model prefix caching silently drops to 0% when the align-mode Mamba checkpoint lands in request-unique tokens · Issue #45238 · vllm-project/vllm · GitHub](https://github.com/vllm-project/vllm/issues/45238)
43. [\[Bug\]: Qwen Tool Calling errors - 3 parser issues · Issue #61206 · vllm-project/vllm · GitHub](https://github.com/vllm-project/vllm/issues/61206)
44. [Google ADK for Go agents (2026): adk-go release version, tools, session service database and human-in-the-loop tool confirmation — Cadenya](https://cadenya.com/other-content/google-adk-for-go-agents)
45. [feat(adk): honour context compaction in the Go runtime by teemow · Pull Request #2790 · kagent-dev/kagent · GitHub](https://github.com/kagent-dev/kagent/pull/2790)
46. [Local Agents with llama.cpp · Hugging Face](https://huggingface.co/docs/hub/agents-local)
47. [How to run Terminal-Bench 2.0](https://www.tbench.ai/docs/run-terminal-bench-2-0)
48. [terminal-bench/terminal-bench-2-1](https://hub.harborframework.com/datasets/terminal-bench/terminal-bench-2-1/latest)
