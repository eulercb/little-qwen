// Scripted OpenAI-compatible streaming stub for offline tests of the extension.
// Serves the replies in order and records every request body (JSONL).
//
//   node stub.mjs <port> <script.json> <requests.jsonl>
//
// A reply: { reasoning?, reasoningChunks?, content?, tool_calls?: [{name, args}],
//            finish?: "stop"|"tool_calls"|"length", delayMs? }
import http from "node:http";
import { appendFileSync, readFileSync } from "node:fs";

const [port, scriptPath, reqLog] = process.argv.slice(2);
const replies = JSON.parse(readFileSync(scriptPath, "utf8"));
let n = 0;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

http
  .createServer(async (req, res) => {
    let body = "";
    for await (const c of req) body += c;
    if (req.method !== "POST") {
      res.writeHead(req.url === "/health" ? 200 : 404, { "Content-Type": "application/json" }).end('{"status":"ok"}');
      return;
    }
    appendFileSync(reqLog, body.replace(/\n/g, " ") + "\n");
    const r = replies[n++] ?? { content: "out of script" };
    res.writeHead(200, { "Content-Type": "text/event-stream" });
    let closed = false;
    res.on("close", () => (closed = true));
    const send = (o) => !closed && res.write(`data: ${JSON.stringify(o)}\n\n`);
    const delta = (d) => send({ id: "x", object: "chat.completion.chunk", choices: [{ index: 0, delta: d }] });
    if (r.reasoning) {
      const parts = r.reasoningChunks ?? 1;
      const size = Math.ceil(r.reasoning.length / parts);
      for (let i = 0; i < parts && !closed; i++) {
        delta({ reasoning_content: r.reasoning.slice(i * size, (i + 1) * size) });
        if (r.delayMs) await sleep(r.delayMs);
      }
    }
    if (closed) return;
    if (r.content) delta({ content: r.content });
    let finish = r.finish ?? "stop";
    if (r.tool_calls) {
      delta({
        tool_calls: r.tool_calls.map((c, i) => ({ index: i, id: `call_${n}_${i}`, type: "function", function: { name: c.name, arguments: JSON.stringify(c.args) } })),
      });
      finish = r.finish ?? "tool_calls";
    }
    send({ id: "x", object: "chat.completion.chunk", choices: [{ index: 0, delta: {}, finish_reason: finish }] });
    send({ id: "x", object: "chat.completion.chunk", choices: [], usage: { prompt_tokens: 100, completion_tokens: 10, total_tokens: 110 } });
    if (!closed) res.end("data: [DONE]\n\n");
  })
  .listen(Number(port), "127.0.0.1");
