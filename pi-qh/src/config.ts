// Config for the qh-on-pi extension. Defaults reproduce harness/it2/config.json;
// every mechanism can be switched off for ablations.
//
//   QH_PI_CONFIG=<file.json>   overrides (same keys as below)
//   QH_PI_OFF=thinkCap,loops   comma-separated mechanisms to disable
//   QH_CHECK=<cmd>             the task's acceptance command (check on finish)
//   QH_PI_TRACE=<file.jsonl>   trace output (default: none)
//   QH_PI_STATUS=<file.json>   final status for the eval runner (default: none)
import { readFileSync } from "node:fs";

export interface QhConfig {
  // L1: cap a turn's reasoning; keep the partial reasoning and act with thinking off.
  thinkBudgetTokens: number;
  thinkCapContinue: boolean;
  // L2: nudge to build/test after this many untested written lines (0 = off).
  testAfterWriteLines: number;
  // L3: on a budget stop, restore the last tree whose build/test passed.
  restoreGreen: boolean;
  fuzzyEdit: boolean;
  verifyWrites: boolean;
  checkOnFinish: boolean;
  maxFinishAttempts: number;
  maxNudges: number;
  maxTurns: number;
  maxWallMinutes: number;
  loopWindow: number;
  loopRepeat: number;
  loopNoProgress: number;
  toolCapTokens: number;
  readMaxLines: number;
  bashDefaultTimeout: number;
  bashMaxTimeout: number;
  qhPrompt: boolean;
  taskCard: boolean;
  planTool: boolean;
  // Chunked execution (see chunks.ts): off by default, not part of it2.
  chunks: boolean;
  chunkMaxTurns: number;
}

export const IT2: QhConfig = {
  thinkBudgetTokens: 8000,
  thinkCapContinue: true,
  testAfterWriteLines: 300,
  restoreGreen: true,
  fuzzyEdit: true,
  verifyWrites: true,
  checkOnFinish: true,
  maxFinishAttempts: 4,
  maxNudges: 3,
  maxTurns: 80,
  maxWallMinutes: 60,
  loopWindow: 8,
  loopRepeat: 3,
  loopNoProgress: 14,
  toolCapTokens: 4000,
  readMaxLines: 400,
  bashDefaultTimeout: 120,
  bashMaxTimeout: 600,
  qhPrompt: true,
  taskCard: true,
  planTool: true,
  chunks: false,
  chunkMaxTurns: 40,
};

const SWITCHES: Record<string, Partial<QhConfig>> = {
  thinkCap: { thinkBudgetTokens: 0 },
  testNudge: { testAfterWriteLines: 0 },
  restoreGreen: { restoreGreen: false },
  fuzzyEdit: { fuzzyEdit: false },
  verifyWrites: { verifyWrites: false },
  checkOnFinish: { checkOnFinish: false },
  loops: { loopRepeat: 0, loopNoProgress: 0 },
  cap: { toolCapTokens: 0 },
  readWindow: { readMaxLines: 0 },
  budget: { maxTurns: 0, maxWallMinutes: 0 },
  qhPrompt: { qhPrompt: false },
  taskCard: { taskCard: false },
  planTool: { planTool: false },
};

export function loadConfig(env: NodeJS.ProcessEnv = process.env): QhConfig {
  let cfg: QhConfig = { ...IT2 };
  if (env.QH_PI_CONFIG) {
    cfg = { ...cfg, ...JSON.parse(readFileSync(env.QH_PI_CONFIG, "utf8")) };
  }
  for (const name of (env.QH_PI_OFF ?? "").split(",").map((s) => s.trim()).filter(Boolean)) {
    if (!SWITCHES[name]) throw new Error(`QH_PI_OFF: unknown mechanism ${name} (known: ${Object.keys(SWITCHES).join(", ")})`);
    cfg = { ...cfg, ...SWITCHES[name] };
  }
  if (env.QH_PI_CHUNKS === "1") cfg.chunks = true;
  return cfg;
}

// Same estimate as qh's ctxmgr.CharsPerToken.
export const CHARS_PER_TOKEN = 3.3;
