// Offline end-to-end tests: the real pi binary + this extension against a
// scripted stub (test/stub.mjs). Each scenario checks one ported mechanism by
// inspecting the requests pi actually sent and the files/status it produced.
//
//   node test/run.mjs [scenario-name ...]
//
// PI_BIN defaults to ~/.cache/qh-pi/node_modules/.bin/pi.
import { spawn, spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, writeFileSync, existsSync } from "node:fs";
import { tmpdir, homedir } from "node:os";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const EXT = join(here, "..", "extensions", "qh.ts");
const PI = process.env.PI_BIN ?? join(homedir(), ".cache/qh-pi/node_modules/.bin/pi");
const MODEL = "mlx-community/Qwen3.8-27B-4bit";
let port = 18300;

async function run(name, { replies, prompt = "Make hello.txt containing hi.", env = {}, setup }) {
  const dir = mkdtempSync(join(tmpdir(), `piqh-${name}-`));
  const work = join(dir, "work");
  mkdirSync(work);
  const sh = (c) => spawnSync("bash", ["-c", c], { cwd: work, encoding: "utf8" });
  if (setup) setup(work, sh);
  sh("git init -q && mkdir -p .git/info && echo '.qh/' >> .git/info/exclude && git add -A && git -c user.email=t@t -c user.name=t commit -qm init --allow-empty");
  const p = port++;
  writeFileSync(join(dir, "script.json"), JSON.stringify(replies));
  const stub = spawn(process.execPath, [join(here, "stub.mjs"), String(p), join(dir, "script.json"), join(dir, "requests.jsonl")], { stdio: "ignore" });
  await new Promise((r) => setTimeout(r, 300));
  const agentDir = join(dir, "pi");
  mkdirSync(agentDir);
  writeFileSync(
    join(agentDir, "models.json"),
    JSON.stringify({
      providers: {
        splash: {
          api: "openai-completions",
          baseUrl: `http://127.0.0.1:${p}/v1`,
          apiKey: "unused",
          compat: { thinkingFormat: "openai", supportsReasoningEffort: true, supportsDeveloperRole: false, supportsStore: false, maxTokensField: "max_tokens" },
          models: [{ id: MODEL, name: "q", reasoning: true, input: ["text"], contextWindow: 131072, maxTokens: 16000, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 } }],
        },
      },
    }),
  );
  const out = spawnSync(PI, ["-p", "--no-session", "--approve", "--model", `splash/${MODEL}`, "--thinking", "medium", "-e", EXT, prompt], {
    cwd: work,
    encoding: "utf8",
    input: "",
    timeout: 120_000,
    env: {
      ...process.env,
      PI_CODING_AGENT_DIR: agentDir,
      PI_OFFLINE: "1",
      NO_PROXY: "127.0.0.1,localhost",
      no_proxy: "127.0.0.1,localhost",
      QH_PI_TRACE: join(dir, "trace.jsonl"),
      QH_PI_STATUS: join(dir, "status.json"),
      ...env,
    },
  });
  stub.kill();
  if (process.env.DEBUG) console.log(`[debug ${name}] dir=${dir}\n${(out.stdout ?? "") + (out.stderr ?? "")}`.slice(0, 4000));
  const lines = (f) => (existsSync(f) ? readFileSync(f, "utf8").split("\n").filter(Boolean).map((l) => JSON.parse(l)) : []);
  return {
    dir,
    work,
    out: (out.stdout ?? "") + (out.stderr ?? ""),
    requests: lines(join(dir, "requests.jsonl")),
    trace: lines(join(dir, "trace.jsonl")),
    status: existsSync(join(dir, "status.json")) ? JSON.parse(readFileSync(join(dir, "status.json"), "utf8")) : {},
    sh: (c) => spawnSync("bash", ["-c", c], { cwd: work, encoding: "utf8" }),
  };
}

const text = (m) => (typeof m.content === "string" ? m.content : (m.content ?? []).map((c) => c.text ?? "").join(""));
const events = (r, name) => r.trace.filter((e) => e.type === "event" && e.name === name);
const write = (path, content) => ({ name: "write", args: { path, content } });
const bash = (command) => ({ name: "bash", args: { command } });

const scenarios = {
  // Prompt, task card, tools, bash timeout, read window, check on finish.
  async basics(t) {
    const r = await run("basics", {
      setup: (w) => writeFileSync(join(w, "big.txt"), Array.from({ length: 1000 }, (_, i) => `line ${i}`).join("\n")),
      env: { QH_CHECK: "test -f hello.txt" },
      replies: [
        { tool_calls: [{ name: "read", args: { path: "big.txt" } }] },
        { tool_calls: [write("hello.txt", "hi\n"), bash("cat hello.txt")] },
        { content: "Done: wrote hello.txt." },
      ],
    });
    const first = r.requests[0];
    t(first.messages[0].role === "system" && text(first.messages[0]).startsWith("# Role\nYou are qh"), "qh system prompt");
    const user = text(first.messages[1]);
    t(user.startsWith("# Task\nMake hello.txt") && user.includes("# Acceptance check") && user.includes("big.txt (") , "task card with check and file tree");
    t(first.tools.map((x) => x.function.name).includes("update_plan"), "update_plan tool registered");
    t(first.reasoning_effort === "medium" && first.max_tokens === 16000, "effort medium, max_tokens 16000");
    const readResult = r.requests[1].messages.find((m) => m.role === "tool");
    t(readResult && text(readResult).includes("line 399") && !text(readResult).includes("line 400\n"), "read window 400 lines");
    t(r.status.status === "finished" && r.status.check_passed === true, `check on finish passed (${JSON.stringify(r.status)})`);
  },

  // L1: reasoning over 8K tokens is cut, kept, and the next turn runs with thinking off.
  async thinkcap(t) {
    const r = await run("thinkcap", {
      env: { QH_CHECK: "test -f hello.txt" },
      replies: [
        { reasoning: "x".repeat(40000), reasoningChunks: 40, delayMs: 20 },
        { tool_calls: [write("hello.txt", "hi\n")] },
        { content: "Done." },
      ],
    });
    t(events(r, "think_budget").length === 1, "think_budget fired once");
    const act = r.requests[1];
    t(act && act.chat_template_kwargs?.enable_thinking === false && act.reasoning_effort === undefined && act.temperature === 0.7, "act turn: thinking off, non-thinking sampling");
    const msgs = act?.messages ?? [];
    const asst = msgs.find((m) => m.role === "assistant");
    t(asst && (asst.reasoning_content ?? "").length > 28000, `partial reasoning kept (${asst ? (asst.reasoning_content ?? "").length : "no assistant"})`);
    t(text(msgs[msgs.length - 1]).includes("that is enough to act on"), "act nudge is the last message");
    const after = r.requests[2];
    t(after && after.reasoning_effort === "medium" && !after.chat_template_kwargs, "thinking back on after the act turn");
    const same = (a, b) => JSON.stringify(a.messages.slice(0, b.messages.length)) === JSON.stringify(b.messages);
    t(after && same(after, act), "history stays append-only after the cut (prefix-stable)");
    t(r.status.check_passed === true, "task completed");
  },

  // Failing acceptance check → feedback and another attempt.
  async checkfail(t) {
    const r = await run("checkfail", {
      env: { QH_CHECK: "grep -q ok hello.txt" },
      replies: [
        { tool_calls: [write("hello.txt", "no\n")] },
        { content: "Done." },
        { tool_calls: [write("hello.txt", "ok\n")] },
        { content: "Fixed, done." },
      ],
    });
    const fb = r.requests[2]?.messages.at(-1);
    t(fb && text(fb).includes("ERROR check_failed"), "check failure fed back to the model");
    t(r.status.check_passed === true && r.status.finish_attempts === 2, `passed on the second attempt (${JSON.stringify(r.status)})`);
  },

  // Output limit hit with no action → nudge, then continue.
  async length(t) {
    const r = await run("length", {
      env: { QH_CHECK: "test -f hello.txt" },
      replies: [
        { reasoning: "thinking...", finish: "length" },
        { tool_calls: [write("hello.txt", "hi\n")] },
        { content: "Done." },
      ],
    });
    t(r.requests[1] && text(r.requests[1].messages.at(-1)).includes("hit the output token limit"), "length nudge sent");
    t(r.status.check_passed === true, "task completed after the nudge");
  },

  // Fuzzy edit: spaces in oldText against a tab-indented file.
  async fuzzy(t) {
    const r = await run("fuzzy", {
      setup: (w) => writeFileSync(join(w, "a.go"), "package a\n\nfunc F() int {\n\tx := 1\n\treturn x\n}\n"),
      replies: [
        { tool_calls: [{ name: "edit", args: { path: "a.go", edits: [{ oldText: "    x := 1\n    return x", newText: "    x := 2\n    return x" }] } }] },
        { content: "Done." },
      ],
    });
    const f = readFileSync(join(r.work, "a.go"), "utf8");
    t(f.includes("\tx := 2\n\treturn x"), `edit applied with the file's tabs (${JSON.stringify(f)})`);
    t(events(r, "fuzzy_edit").length === 1, "fuzzy_edit traced");
  },

  // L2: a large untested write gets the build/test note; a test run resets it.
  async testnudge(t) {
    const big = Array.from({ length: 320 }, (_, i) => `// ${i}`).join("\n");
    const r = await run("testnudge", {
      replies: [{ tool_calls: [write("big.go", big)] }, { content: "Done." }],
    });
    const res = r.requests[1]?.messages.find((m) => m.role === "tool");
    t(res && text(res).includes("lines written since the last build/test run"), "L2 note appended to the write result");
  },

  // L3 + budget: a passing check snapshots the tree; a budget stop with a broken tree restores it.
  async restore(t) {
    const r = await run("restore", {
      setup: (w) => writeFileSync(join(w, "Makefile"), "check:\n\tgrep -q good a.txt\n"),
      env: { QH_PI_CONFIG: writeCfg({ maxTurns: 3 }) },
      replies: [
        { tool_calls: [write("a.txt", "good\n")] },
        { tool_calls: [bash("make -s check")] },
        { tool_calls: [{ name: "edit", args: { path: "a.txt", edits: [{ oldText: "good", newText: "bad" }] } }] },
        { tool_calls: [bash("echo never")] },
      ],
    });
    t(r.status.status === "max_turns", `stopped by the turn budget (${r.status.status})`);
    t(readFileSync(join(r.work, "a.txt"), "utf8") === "good\n", "last green tree restored");
    t(events(r, "restore_green").length === 1, "restore_green traced");
  },

  // Loop detector: the same call three times.
  async loops(t) {
    const r = await run("loops", {
      replies: [{ tool_calls: [bash("ls")] }, { tool_calls: [bash("ls")] }, { tool_calls: [bash("ls")] }, { content: "Done." }],
    });
    const res = r.requests[3]?.messages.at(-1);
    t(res && text(res).includes("LOOP WARNING"), "loop warning on the third identical call");
  },

  // Output cap: long output is cut to head+tail and spilled to .qh/spill.
  async cap(t) {
    const r = await run("cap", {
      replies: [{ tool_calls: [bash("seq 1 3000 | sed \"s/$/ xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx/\"")] }, { content: "Done." }],
    });
    const res = r.requests[1]?.messages.at(-1);
    t(res && text(res).includes("lines omitted") && text(res).length < 15000, "output capped inline");
    t(r.sh("ls .qh/spill/ | wc -l").stdout.trim() === "1", "full output spilled");
  },

  // Chunked execution: plan → chunk 1 → chunk 2 → final; each chunk starts with a fresh context.
  async chunks(t) {
    const r = await run("chunks", {
      env: { QH_PI_CHUNKS: "1", QH_CHECK: "test -f a.txt && test -f b.txt" },
      prompt: "Create a.txt and b.txt.",
      replies: [
        { tool_calls: [bash("ls")] },
        { tool_calls: [{ name: "plan_chunks", args: { chunks: [{ title: "A", goal: "create a.txt", done_when: "test -f a.txt" }, { title: "B", goal: "create b.txt", done_when: "test -f b.txt" }], notes: "plain files" } }] },
        { tool_calls: [{ name: "chunk_done", args: { report: "too early" } }] },
        { tool_calls: [write("a.txt", "a\n")] },
        { tool_calls: [{ name: "chunk_done", args: { report: "a.txt created" } }] },
        { tool_calls: [write("b.txt", "b\n")] },
        { tool_calls: [{ name: "chunk_done", args: { report: "b.txt created" } }] },
        { tool_calls: [bash("ls")] },
        { content: "All done." },
      ],
    });
    const q = r.requests;
    t(text(q[0].messages[1]).includes("call plan_chunks"), "plan instruction in the first card");
    const c1 = q[2]?.messages ?? [];
    t(c1.length === 2 && text(c1[1]).includes("[this] 1. A") && text(c1[1]).includes("[later] 2. B"), `chunk 1 starts fresh (${c1.length} messages)`);
    t(text(q[3].messages.at(-1)).includes("done_when failed"), "done_when failure keeps the chunk open");
    const c2 = q[5]?.messages ?? [];
    t(c2.length === 2 && text(c2[1]).includes("[done] 1. A") && text(c2[1]).includes("a.txt created"), "chunk 2 starts fresh with chunk 1's report");
    const fin = q[7]?.messages ?? [];
    t(fin.length === 2 && text(fin[1]).includes("All chunks are done"), "final phase card");
    t(q.every((x) => text(x.messages[1]).startsWith("# Task\nCreate a.txt")), "task card is the stable first user message");
    t(r.status.check_passed === true, `finished with the check passing (${JSON.stringify(r.status)})`);
    t(existsSync(join(r.work, ".qh/CHUNKS.md")) && existsSync(join(r.work, ".qh/NOTES.md")), "plan and notes on disk");
  },

  // Chunk mode: the model may not finish while chunks remain.
  async chunknudge(t) {
    const r = await run("chunknudge", {
      env: { QH_PI_CHUNKS: "1" },
      replies: [
        { tool_calls: [{ name: "plan_chunks", args: { chunks: [{ title: "A", goal: "create a.txt" }] } }] },
        { content: "I am done." },
        { tool_calls: [{ name: "chunk_done", args: { report: "nothing" } }] },
        { content: "Done." },
      ],
    });
    t(r.requests[2] && text(r.requests[2].messages.at(-1)).includes("still open"), "finishing with an open chunk is nudged");
  },
};

function writeCfg(o) {
  const f = join(mkdtempSync(join(tmpdir(), "piqh-cfg-")), "cfg.json");
  writeFileSync(f, JSON.stringify(o));
  return f;
}

const only = process.argv.slice(2);
let failed = 0;
for (const [name, fn] of Object.entries(scenarios)) {
  if (only.length && !only.includes(name)) continue;
  const results = [];
  try {
    await fn((ok, msg) => results.push([ok, msg]));
  } catch (e) {
    results.push([false, `threw: ${e.stack ?? e}`]);
  }
  for (const [ok, msg] of results) {
    console.log(`${ok ? "ok  " : "FAIL"} ${name}: ${msg}`);
    if (!ok) failed++;
  }
}
console.log(failed ? `${failed} check(s) failed` : "all checks passed");
process.exit(failed ? 1 : 0);
