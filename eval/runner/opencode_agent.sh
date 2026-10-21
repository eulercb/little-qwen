#!/usr/bin/env bash
# External baseline agent: stock OpenCode pointed at the same endpoint.
# Used by: qh eval --agent eval/runner/opencode_agent.sh
# Needs OPENCODE_BIN (default: opencode on PATH). Runs in the task workdir.
set -u
bin=${OPENCODE_BIN:-opencode}
cat > opencode.json <<JSON
{
  "\$schema": "https://opencode.ai/config.json",
  "provider": {
    "splash": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "splash",
      "options": {"baseURL": "${REMOTE_LLM_ENDPOINT%/}/v1"},
      "models": {"${QH_MODEL:-mlx-community/Qwen3.8-27B-4bit}": {"name": "Qwen3.8-27B", "limit": {"context": 131072, "output": 16000}}}
    }
  },
  "model": "splash/${QH_MODEL:-mlx-community/Qwen3.8-27B-4bit}",
  "autoupdate": false,
  "share": "disabled"
}
JSON
echo "opencode.json" >> .git/info/exclude
export OPENCODE_DISABLE_MODELS_FETCH=true OPENCODE_DISABLE_AUTOUPDATE=true
# opencode run sometimes does not exit after the session ends; bound it.
timeout "${QH_TIMEOUT_S:-3600}" "$bin" run --print-logs "$(cat "$QH_PROMPT_FILE")" 2>&1 | grep -v -E '^(DEBUG|INFO .*service=bus)' | tail -c 200000
exit 0
