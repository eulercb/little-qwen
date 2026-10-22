#!/usr/bin/env bash
# One-command resume of the paused experiments (see STATUS.md).
#
#   eval/runner/resume.sh https://<tunnel>.trycloudflare.com [--no-opencode]
#
# Builds qh, runs the offline tests, checks the endpoint (models + a chat
# round-trip), installs OpenCode for the comparator item unless --no-opencode,
# and starts the background eval queue (eval/queue.txt). Safe to re-run: it
# refuses to start a second queue.
set -euo pipefail
cd "$(dirname "$0")/../.."

ep="${1:-${REMOTE_LLM_ENDPOINT:-}}"
if [ -z "$ep" ]; then
  echo "usage: $0 https://<tunnel>.trycloudflare.com [--no-opencode]" >&2
  exit 2
fi
ep="${ep%/}"
case "$ep" in http*) ;; *) ep="https://$ep" ;; esac
export REMOTE_LLM_ENDPOINT="$ep"

if [ -f eval/queue.pid ] && kill -0 "$(cat eval/queue.pid)" 2>/dev/null; then
  echo "queue already running (pid $(cat eval/queue.pid)); nothing to do"
  exit 0
fi
rm -f eval/queue.pid eval/.eval.lock

echo "== build + offline tests"
go build -o bin/qh ./cmd/qh
CGO_ENABLED=0 go build -o bin/qh-static ./cmd/qh   # copied into Terminal-Bench containers
go test ./internal/... >/dev/null
./bin/qh validate harness/base >/dev/null

echo "== endpoint $ep"
if ! curl -sf -m 30 "$ep/v1/models" | grep -q Qwen3.8; then
  echo "endpoint not serving the model (a 403 means the server needs --allowed-host for this tunnel host)" >&2
  curl -s -m 30 "$ep/v1/models" | head -c 300 >&2 || true
  exit 1
fi
reply=$(curl -sf -m 180 "$ep/v1/chat/completions" -H 'Content-Type: application/json' \
  -d '{"model":"mlx-community/Qwen3.8-27B-4bit","messages":[{"role":"user","content":"Reply with exactly: pong"}],"max_tokens":200}' \
  | python3 -c 'import sys,json; print(json.load(sys.stdin)["choices"][0]["message"]["content"])')
echo "chat round-trip: $reply"

if [ "${2:-}" != "--no-opencode" ] && grep -q OPENCODE_BIN eval/queue.txt; then
  oc="${OPENCODE_BIN:-$HOME/.cache/qh-opencode/node_modules/.bin/opencode}"
  if [ ! -x "$oc" ]; then
    echo "== installing OpenCode (comparator) into $HOME/.cache/qh-opencode"
    mkdir -p "$HOME/.cache/qh-opencode"
    (cd "$HOME/.cache/qh-opencode" && npm install --silent opencode-ai@latest >/dev/null)
    oc="$HOME/.cache/qh-opencode/node_modules/.bin/opencode"
  fi
  export OPENCODE_BIN="$oc"
  echo "OpenCode: $OPENCODE_BIN"
fi

if grep -q "qh tbench" eval/queue.txt; then
  echo "== docker + Terminal-Bench prerequisites"
  eval/runner/docker-up.sh
  eval/runner/tb-cache.sh
  [ -d /home/user/laude-institute/terminal-bench-2 ] || GIT_LFS_SKIP_SMUDGE=1 git clone --depth 1 \
    https://github.com/laude-institute/terminal-bench-2 /home/user/laude-institute/terminal-bench-2
  git -C /home/user/laude-institute/terminal-bench-2 fetch -q --depth 1 origin 2fd12b88aafdd04a52c298e3940bcb189f9766d6 2>/dev/null &&
    git -C /home/user/laude-institute/terminal-bench-2 checkout -q 2fd12b88aafdd04a52c298e3940bcb189f9766d6 2>/dev/null || true
fi

echo "== remaining queue items"
grep -v -E '^\s*(#|$)' eval/queue.txt | sed -E 's/ >.*//' | nl

nohup eval/runner/queue.sh >> eval/queue.log 2>&1 &
sleep 2
echo "queue started (pid $(cat eval/queue.pid)); log: eval/queue.log; results are committed per item"
