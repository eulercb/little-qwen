// JSONL trace in the spirit of qh's internal/trace, so pi runs can be compared
// with qh runs (turn usage, harness interventions, final status).
import { appendFileSync, writeFileSync } from "node:fs";

export class Trace {
  constructor(private path: string | undefined, private statusPath: string | undefined) {}

  event(name: string, data: Record<string, unknown> = {}): void {
    if (!this.path) return;
    try {
      appendFileSync(this.path, JSON.stringify({ type: "event", t: new Date().toISOString(), name, ...data }) + "\n");
    } catch {
      // tracing must never break a run
    }
  }

  turn(data: Record<string, unknown>): void {
    if (!this.path) return;
    try {
      appendFileSync(this.path, JSON.stringify({ type: "turn", t: new Date().toISOString(), ...data }) + "\n");
    } catch {}
  }

  status(data: Record<string, unknown>): void {
    if (!this.statusPath) return;
    try {
      writeFileSync(this.statusPath, JSON.stringify(data, null, 2) + "\n");
    } catch {}
  }
}

/** Canonical JSON (sorted keys), as qh's repair.Hash canonicalises args. */
export function canonical(v: unknown): string {
  if (Array.isArray(v)) return "[" + v.map(canonical).join(",") + "]";
  if (v && typeof v === "object") {
    const o = v as Record<string, unknown>;
    return "{" + Object.keys(o).sort().map((k) => JSON.stringify(k) + ":" + canonical(o[k])).join(",") + "}";
  }
  return JSON.stringify(v);
}
