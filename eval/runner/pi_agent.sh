#!/usr/bin/env bash
# External comparator agents built on pi (pi.dev), pointed at the same endpoint.
#   qh eval --agent eval/runner/pi_agent.sh            # vanilla pi
#   qh eval --agent "eval/runner/pi_agent.sh little"   # little-coder (pi + extensions)
# Binaries: PI_BIN (default ~/.cache/qh-pi/node_modules/.bin/pi) and LITTLE_CODER_BIN
# (default ~/.cache/qh-littlecoder/node_modules/.bin/little-coder); resume.sh installs both.
# Config lives in a throwaway dir, never in the task workdir (the diff is scored).
# Thinking: medium (same as qh's default effort); everything else is the agent's
# own default, i.e. the out-of-the-box experience.
set -u
flavor=${1:-pi}
model=${QH_MODEL:-mlx-community/Qwen3.8-27B-4bit}
cfg=$(mktemp -d)
trap 'rm -rf "$cfg"' EXIT
models_json() {
  cat <<JSON
{
  "providers": {
    "splash": {
      "api": "openai-completions",
      "baseUrl": "${REMOTE_LLM_ENDPOINT%/}/v1",
      "apiKey": "unused",
      "compat": {"thinkingFormat": "openai", "supportsReasoningEffort": true, "supportsDeveloperRole": false, "supportsStore": false, "maxTokensField": "max_tokens"},
      "models": [{"id": "$model", "name": "Qwen3.8-27B (splash MLX)", "reasoning": true, "input": ["text"],
                  "contextWindow": 131072, "maxTokens": 16384,
                  "cost": {"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0}}]
    }
  }
}
JSON
}
# Node fetch ignores HTTPS_PROXY unless asked (container egress goes through a proxy).
export NODE_USE_ENV_PROXY=1
prompt=$(cat "$QH_PROMPT_FILE")
case "$flavor" in
  pi)
    bin=${PI_BIN:-$HOME/.cache/qh-pi/node_modules/.bin/pi}
    export PI_CODING_AGENT_DIR="$cfg/pi" PI_OFFLINE=1
    mkdir -p "$PI_CODING_AGENT_DIR"
    models_json > "$PI_CODING_AGENT_DIR/models.json"
    timeout "${QH_TIMEOUT_S:-3600}" "$bin" -p --no-session --approve --model "splash/$model" --thinking medium "$prompt" </dev/null 2>&1 | tail -c 200000
    ;;
  little)
    bin=${LITTLE_CODER_BIN:-$HOME/.cache/qh-littlecoder/node_modules/.bin/little-coder}
    export XDG_CONFIG_HOME="$cfg/xdg" PI_CODING_AGENT_DIR="$cfg/pi" PI_OFFLINE=1 \
      LITTLE_CODER_PERMISSION_MODE=accept-all LITTLE_CODER_NO_UPDATE_CHECK=1
    mkdir -p "$XDG_CONFIG_HOME/little-coder" "$PI_CODING_AGENT_DIR"
    models_json > "$XDG_CONFIG_HOME/little-coder/models.json"
    timeout "${QH_TIMEOUT_S:-3600}" "$bin" -p --no-session --model "splash/$model" --thinking medium "$prompt" </dev/null 2>&1 | tail -c 200000
    ;;
  *) echo "unknown flavor $flavor" >&2; exit 2 ;;
esac
exit 0
