#!/usr/bin/env bash
# Sequential eval queue. Each non-comment line of eval/queue.txt is a command run
# from the repo root; finished lines are moved to eval/queue.done. Safe to restart:
# qh eval skips runs that already have result.json.
# Usage: nohup eval/runner/queue.sh >> eval/queue.log 2>&1 &
# Running? test -f eval/queue.pid && kill -0 $(cat eval/queue.pid)   (don't use pgrep -f: it matches itself)
cd "$(dirname "$0")/../.."
q=eval/queue.txt; done_f=eval/queue.done
pidf=eval/queue.pid
if [ -f "$pidf" ] && kill -0 "$(cat "$pidf")" 2>/dev/null; then
  echo "queue already running (pid $(cat "$pidf"))"; exit 1
fi
echo $$ > "$pidf"
trap 'rm -f "$pidf"' EXIT
touch "$q" "$done_f"
while true; do
  line=$(grep -v -E '^\s*(#|$)' "$q" | head -1)
  if [ -z "$line" ]; then echo "$(date -u +%FT%TZ) queue empty"; exit 0; fi
  # Never overlap with an eval started elsewhere: qh eval holds eval/.eval.lock (its pid).
  while [ -f eval/.eval.lock ] && kill -0 "$(cat eval/.eval.lock 2>/dev/null)" 2>/dev/null; do sleep 30; done
  # Wait for a healthy endpoint before starting (tunnels come and go).
  ep="${QH_ENDPOINT:-$REMOTE_LLM_ENDPOINT}"; ep="${ep%/}"
  until curl -sf -m 20 "$ep/health" >/dev/null 2>&1; do
    echo "$(date -u +%FT%TZ) endpoint $ep not healthy; waiting"; sleep 120
  done
  echo "$(date -u +%FT%TZ) START $line"
  bash -c "$line"
  rc=$?
  echo "$(date -u +%FT%TZ) END rc=$rc $line"
  if [ "$rc" -ne 0 ]; then
    # Keep the line (e.g. endpoint outage); retry after the health gate.
    echo "$(date -u +%FT%TZ) item failed (rc=$rc); keeping it in the queue"; sleep 60; continue
  fi
  # remove the first occurrence of this exact line from the queue
  python3 - "$q" "$line" <<'PY'
import sys
p, line = sys.argv[1], sys.argv[2]
lines = open(p).read().split("\n")
for i, l in enumerate(lines):
    if l == line:
        del lines[i]; break
open(p, "w").write("\n".join(lines))
PY
  echo "$(date -u +%FT%TZ) rc=$rc $line" >> "$done_f"
  # Persist results so an interrupted session loses nothing (retry if the index is busy).
  for try in 1 2 3; do
    git add eval/results eval/queue.txt eval/queue.done >/dev/null 2>&1 &&
      git commit -q -m "eval results: $(echo "$line" | grep -o -- '--out [^ ]*' | cut -d' ' -f2)" >/dev/null 2>&1
    git push -q >/dev/null 2>&1 && break
    sleep $((try * 5))
  done
done
