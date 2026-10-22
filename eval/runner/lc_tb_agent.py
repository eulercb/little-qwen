#!/usr/bin/env python3
"""little-coder as a Terminal-Bench agent for `qh tbench --agent`.

  ./bin/qh tbench --agent eval/runner/lc_tb_agent.py --only @eval/tbench/subset.txt --out eval/results/tbC-littlecoder

This mirrors little-coder's own Harbor adapter (benchmarks/harbor_adapter/little_coder_agent.py):
- the same prompt;
- tools limited to ShellSession/ShellSessionCwd/ShellSessionReset;
- a 40-turn cap and benchmark=terminal_bench;
- a stateful shell (cwd tracked by the proxy, output trimmed to 200 lines).

Harbor's env.exec is replaced by `docker exec` on $QH_CONTAINER. It drives `pi --mode rpc` through
little-coder's benchmarks/rpc_client.py (clone at $LC_REPO), with pi and the extensions taken from the npm
install in ~/.cache/qh-littlecoder. Output formatting is adapted from little-coder (Apache-2.0).
"""
import json
import os
import re
import subprocess
import sys
import tempfile
import uuid
from pathlib import Path

LC_REPO = Path(os.environ.get("LC_REPO", "/home/user/itayinbarr/little-coder"))
LC_INSTALL = Path(os.environ.get("LC_INSTALL", Path.home() / ".cache/qh-littlecoder/node_modules"))
MODEL = os.environ.get("QH_MODEL", "mlx-community/Qwen3.8-27B-4bit")

sys.path.insert(0, str(LC_REPO / "benchmarks"))
import rpc_client  # noqa: E402

rpc_client.REPO_ROOT = LC_INSTALL / "little-coder"  # extensions + AGENTS.md of the installed release
rpc_client.PI_BIN = LC_INSTALL / ".bin" / "pi"

ANSI_RE = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")
MAX_LINES = 200


def format_output(out: str, code: int, cwd: str, timed_out: bool) -> str:
    lines = ANSI_RE.sub("", out or "").replace("\r", "").split("\n")
    deduped, last, dup = [], None, 0
    for ln in lines:
        if ln == last:
            dup += 1
            continue
        if dup:
            deduped.append(f"  [... {dup} duplicate line(s) collapsed ...]")
        dup = 0
        deduped.append(ln)
        last = ln
    if dup:
        deduped.append(f"  [... {dup} duplicate line(s) collapsed ...]")
    bits = [f"exit={code}", f"cwd={cwd}", f"timed_out={'true' if timed_out else 'false'}"]
    if len(deduped) > MAX_LINES:
        head, tail = MAX_LINES // 2, MAX_LINES // 4
        deduped = deduped[:head] + [f"  [... {len(deduped) - head - tail} lines truncated ...]"] + deduped[-tail:]
        bits.append("output_truncated=true")
    body = "\n".join(deduped)
    footer = "[" + " ".join(bits) + "]"
    return f"{body}\n{footer}" if body else footer


class DockerShell:
    """Stateful shell over stateless `docker exec` (cwd carried between calls)."""

    def __init__(self, container: str, cwd: str):
        self.container, self.home, self.cwd = container, cwd, cwd

    def run(self, command: str, timeout: int) -> str:
        sentinel = f"__LC_END_{uuid.uuid4().hex[:8]}__"
        wrapped = f"cd {self.cwd} 2>/dev/null; {{ {command} ; }} ; __rc=$? ; printf '\\n{sentinel}:%d:' $__rc ; pwd"
        try:
            p = subprocess.run(["docker", "exec", self.container, "bash", "-c", wrapped],
                               capture_output=True, text=True, errors="replace", timeout=timeout)
        except subprocess.TimeoutExpired as e:
            out = (e.stdout or b"").decode(errors="replace") if isinstance(e.stdout, bytes) else (e.stdout or "")
            return format_output(out + "\ncommand timed out", -1, self.cwd, True)
        out, code = p.stdout, p.returncode
        marker = out.rfind(sentinel + ":")
        if marker >= 0:
            parts = out[marker + len(sentinel) + 1:].split(":", 1)
            try:
                code = int(parts[0])
            except ValueError:
                pass
            if len(parts) > 1:
                cwd = parts[1].lstrip("\r\n").split("\n")[0].strip()
                if cwd:
                    self.cwd = cwd
            out = out[:marker].rstrip()
        if p.stderr:
            out += "\n[stderr]\n" + p.stderr
        return format_output(out, code, self.cwd, False)

    def reset(self) -> str:
        self.cwd = self.home
        return f"shell reset (cwd → {self.home})"


def models_override(cfg: Path) -> None:
    d = cfg / "little-coder"
    d.mkdir(parents=True, exist_ok=True)
    (d / "models.json").write_text(json.dumps({"providers": {"splash": {
        "api": "openai-completions",
        "baseUrl": os.environ["REMOTE_LLM_ENDPOINT"].rstrip("/") + "/v1",
        "apiKey": "unused",
        "models": [{"id": MODEL, "name": "Qwen3.8-27B (splash MLX)", "reasoning": True, "input": ["text"],
                    "contextWindow": 131072, "maxTokens": 16384,
                    "cost": {"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0}}]}}}))


def main() -> int:
    container, wd = os.environ["QH_CONTAINER"], os.environ.get("QH_WORKDIR", "/app")
    instruction = Path(os.environ["QH_PROMPT_FILE"]).read_text()
    timeout = int(os.environ.get("QH_TIMEOUT_S", "3600"))
    shell = DockerShell(container, wd)

    def handler(payload: dict) -> str:
        op = payload.get("op")
        if op == "run":
            return shell.run(payload.get("command", ""), int(payload.get("timeout", 30)))
        if op == "reset":
            return shell.reset()
        return f"Error: unknown ShellSession op '{op}'"

    prompt = (
        "You are solving a Terminal-Bench 2.0 task inside a Linux container.\n"
        "The ONLY way to interact with the container is the ShellSession tool; "
        "its cwd persists between calls (tracked by the adapter).\n"
        f"Default working directory is {wd}.\n"
        "File tools like Read/Write/Edit are NOT available — use shell commands "
        "(cat, sed -i, heredoc 'cat > file <<EOF') through ShellSession instead.\n\n"
        f"TASK:\n{instruction}\n\n"
        "When the task is complete, stop calling tools and say 'done'."
    )
    with tempfile.TemporaryDirectory() as tmp:
        tmp = Path(tmp)
        models_override(tmp / "xdg")
        env = {"XDG_CONFIG_HOME": str(tmp / "xdg"), "PI_CODING_AGENT_DIR": str(tmp / "pi"),
               "PI_OFFLINE": "1", "NODE_USE_ENV_PROXY": "1", "LITTLE_CODER_NO_UPDATE_CHECK": "1",
               # As in its aider/gaia runners: without it the gate refuses `cat > file`, which the TB prompt asks for.
               "LITTLE_CODER_PERMISSION_MODE": "accept-all"}
        (tmp / "pi").mkdir()
        rpc = rpc_client.PiRpc(model=f"splash/{MODEL}", cwd=str(tmp), benchmark="terminal_bench",
                               allowed_tools=["ShellSession", "ShellSessionCwd", "ShellSessionReset"],
                               session_id=f"qh-{uuid.uuid4().hex[:10]}", tb_mode=True, max_turns=40,
                               env=env, tb_shell_handler=handler)
        try:
            res = rpc.prompt_and_collect(prompt, timeout=timeout)
            print(f"turns={res.turn_count} tool_calls={len(res.tool_calls)} compactions={res.compaction_events} "
                  f"ended={res.agent_ended}")
            for tc in res.tool_calls:
                print(f">> {tc['name']} {json.dumps(tc.get('args', {}))[:600]}")
                print(f"<< {(tc.get('result_text') or '')[:600]}")
            print("=== assistant text ===\n" + res.assistant_text[-4000:])
            notes = rpc.notifications()
            if notes:
                print(f"=== notifications ({len(notes)}) ===")
                for n in notes:
                    print(f"[{n.get('notifyType', 'info')}] {n.get('message', '')}")
            if rpc.stderr():
                print("=== pi stderr ===\n" + rpc.stderr()[-4000:])
        finally:
            rpc.close(3)
    return 0


if __name__ == "__main__":
    sys.exit(main())
