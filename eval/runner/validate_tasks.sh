#!/usr/bin/env bash
# Validates eval tasks: the hidden check must fail on the starting repo and
# pass (together with the visible check) once solution/ is applied.
# Usage: eval/runner/validate_tasks.sh [task-id...]
set -u
root=$(cd "$(dirname "$0")/../tasks" && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export GOFLAGS=-mod=mod GOTOOLCHAIN=local GOPROXY=off
ids=("$@")
[ ${#ids[@]} -eq 0 ] && ids=($(ls "$root"))
rc=0
for id in "${ids[@]}"; do
  t="$root/$id"; [ -f "$t/task.json" ] || continue
  check=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["check"])' "$t/task.json")
  hidden=$(python3 -c 'import json,sys;d=json.load(open(sys.argv[1]));print(d.get("hidden_check") or d["check"])' "$t/task.json")
  setup=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1])).get("setup",""))' "$t/task.json")
  w="$tmp/$id-base"; mkdir -p "$w"; cp -r "$t/repo/." "$w/" 2>/dev/null
  (cd "$w" && [ -n "$setup" ] && bash -c "$setup" >/dev/null 2>&1)
  cp -r "$t/hidden/." "$w/"
  if (cd "$w" && timeout 300 bash -c "$hidden" >/dev/null 2>&1); then base=PASS; else base=fail; fi
  w2="$tmp/$id-sol"; mkdir -p "$w2"; cp -r "$t/repo/." "$w2/" 2>/dev/null
  (cd "$w2" && [ -n "$setup" ] && bash -c "$setup" >/dev/null 2>&1)
  cp -r "$t/solution/." "$w2/"
  if (cd "$w2" && timeout 300 bash -c "$check" >"$tmp/vis.log" 2>&1); then vis=pass; else vis=FAIL; fi
  cp -r "$t/hidden/." "$w2/"
  if (cd "$w2" && timeout 300 bash -c "$hidden" >"$tmp/hid.log" 2>&1); then sol=pass; else sol=FAIL; fi
  status=OK
  if [ "$base" != fail ] || [ "$vis" != pass ] || [ "$sol" != pass ]; then status=BAD; rc=1; fi
  echo "$status $id: base-hidden=$base solution-visible=$vis solution-hidden=$sol"
  if [ "$status" = BAD ]; then cat "$tmp/vis.log" "$tmp/hid.log" | tail -n 30 | sed 's/^/    /'; fi
done
exit $rc
