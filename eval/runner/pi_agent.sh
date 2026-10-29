#!/usr/bin/env bash
# External comparator agents built on pi (pi.dev), pointed at the same endpoint.
#   qh eval --agent eval/runner/pi_agent.sh            # vanilla pi
#   qh eval --agent "eval/runner/pi_agent.sh little"   # little-coder (pi + extensions)
#   qh eval --agent "eval/runner/pi_agent.sh qh"       # pi + pi-qh (qh it2 ported as a pi extension)
#   qh eval --agent "eval/runner/pi_agent.sh qh-chunks" # pi + pi-qh with chunked execution
# pi-qh reads QH_CHECK and the raw prompt (it builds qh's task card itself), writes its
# trace and agent_status.json into $QH_AGENT_OUT, and takes QH_PI_CONFIG / QH_PI_OFF.
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
  qh|qh-chunks)
    bin=${PI_BIN:-$HOME/.cache/qh-pi/node_modules/.bin/pi}
    ext="$(cd "$(dirname "$0")/../.." && pwd)/pi-qh/extensions/qh.ts"
    export PI_CODING_AGENT_DIR="$cfg/pi" PI_OFFLINE=1 \
      QH_PI_TRACE="${QH_AGENT_OUT:-$cfg}/trace.jsonl" QH_PI_STATUS="${QH_AGENT_OUT:-$cfg}/agent_status.json"
    [ "$flavor" = qh-chunks ] && export QH_PI_CHUNKS=1
    mkdir -p "$PI_CODING_AGENT_DIR"
    models_json | sed 's/"maxTokens": 16384/"maxTokens": 16000/' > "$PI_CODING_AGENT_DIR/models.json"   # qh's max_tokens
    prompt=$(cat "${QH_RAW_PROMPT_FILE:-$QH_PROMPT_FILE}")
    timeout "${QH_TIMEOUT_S:-3600}" "$bin" -p --no-session --approve --model "splash/$model" --thinking medium -e "$ext" "$prompt" </dev/null 2>&1 | tail -c 200000
    ;;
  *) echo "unknown flavor $flavor" >&2; exit 2 ;;
esac
# Report a timeout (exit 124) so qh records status exit_124 instead of "finished".
[ "${PIPESTATUS[0]}" = 124 ] && exit 124
exit 0
