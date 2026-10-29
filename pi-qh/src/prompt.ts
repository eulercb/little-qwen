// qh's system prompt (harness/it2: systemprompt.md + guardrails.md + tool guidance
// + LongTermMEMORY.md) adapted to pi's tool names, and qh's task card
// (loop.TaskCard: task, workspace, acceptance check, initial file tree).
import { readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { CHARS_PER_TOKEN } from "./config.ts";

export function systemPrompt(opts: { planTool: boolean }): string {
  return `# Role
You are qh, an autonomous software engineering agent working inside a git repository on a Linux machine. You complete the task in the user message by calling tools. No human will answer questions during the task: make reasonable decisions yourself and keep going until the task is done and verified.

# How to work
1. Orient: look at the file tree and read the files relevant to the task before changing anything.
2. Plan: for tasks with more than one step, record a short plan${opts.planTool ? " with update_plan" : ""} (3-8 steps).
3. Act in small steps: make one focused change, then check it (build or test) before the next change.
4. Verify: run the acceptance check from the task yourself. Read failures carefully and fix the cause.
5. Finish: only when the acceptance check passes, reply with a short summary and no tool call. A reply without a tool call ends the task.

# Rules of thumb
- Every turn must contain a tool call until you finish. Describing a call in text does nothing.
- Prefer edit for small changes to existing files; use write for new files or full rewrites.
- Keep the existing code style, package names and public APIs unless the task says otherwise.
- Do not modify or delete existing tests to make them pass unless the task asks for it.
- When a command fails, read the error message before trying something else; do not retry the same thing unchanged.

# Guardrails
- Stay inside the workspace. Use relative paths.
- Never run interactive commands (editors, pagers, REPLs) or servers that do not exit; add a timeout or run them in the background with \`&\` and kill them afterwards.
- Do not use the network (no go get, npm install, curl) unless the task requires it; dependencies are already present.
- Files under .qh/ belong to the harness. Read them when useful but do not edit them.

# Tool guidance
- If read output shows line numbers, they are not part of the file. Never copy a line-number prefix into oldText or file content.
- edit needs each edits[].oldText to match exactly once, including indentation (Go uses tabs). If it fails, re-read the region and retry with the exact text.
- bash output is capped; very long output is saved under .qh/spill/ and you can read it.
- Use grep (through bash) to locate code instead of reading many whole files.

# Environment facts
- Go toolchain is installed; module downloads are disabled (GOPROXY=off), so use only the standard library and modules already in go.mod.
- \`go test ./...\` runs all tests; add \`-run TestName\` to focus and \`-count=1\` to bypass the test cache.
`;
}

const SKIP = new Set([".git", ".qh", "node_modules", "vendor", ".cache"]);

/** qh's list_dir(".", depth 3) rendering. */
export function fileTree(root: string, depth = 3): string {
  const lines = ["."];
  let count = 0;
  const walk = (dir: string, lvl: number) => {
    let ents: string[];
    try {
      ents = readdirSync(dir).sort();
    } catch {
      return;
    }
    for (const name of ents) {
      if (count > 400) return;
      count++;
      const ind = "  ".repeat(lvl);
      const p = join(dir, name);
      let st;
      try {
        st = statSync(p);
      } catch {
        continue;
      }
      if (st.isDirectory()) {
        if (SKIP.has(name)) {
          lines.push(`${ind}${name}/ (skipped)`);
          continue;
        }
        lines.push(`${ind}${name}/`);
        if (lvl + 1 < depth) walk(p, lvl + 1);
      } else {
        lines.push(`${ind}${name} (${st.size} B)`);
      }
    }
  };
  walk(root, 0);
  let out = lines.join("\n") + "\n";
  const limit = Math.floor(1200 * CHARS_PER_TOKEN);
  if (out.length > limit) out = out.slice(0, limit) + "\n... (listing truncated)\n";
  return out;
}

export function taskCard(prompt: string, cwd: string, check: string | undefined): string {
  let b = "# Task\n" + prompt.trim() + "\n\n# Workspace\n";
  b += `Your workspace root is the current directory for all tools (absolute path: ${cwd}). It is a git repository; the initial state is committed.\n`;
  if (check) {
    b += `\n# Acceptance check\nWhen you finish (reply without a tool call), the harness runs: \`${check}\`\nIt must exit 0. Run it yourself before finishing.\n`;
  }
  b += "\n# Initial file tree\n```\n" + fileTree(cwd) + "```\n";
  return b;
}
