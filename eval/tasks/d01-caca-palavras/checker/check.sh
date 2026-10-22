#!/usr/bin/env bash
# Hidden checker for d01-caca-palavras. Run in the agent's workdir with QH_TASK_DIR set.
# Prints PASS/FAIL lines per item and a final "SCORE <passed> <total>" (score.py).
set -u
C="$QH_TASK_DIR/checker"
log=$(mktemp)
{
python3 "$C/static.py" "$QH_TASK_DIR"
[ -d node_modules ] || timeout 600 npm install --silent >/dev/null 2>&1
if timeout 900 npm run check >/tmp/zzh-npmcheck.log 2>&1; then echo "PASS npm-check"; else echo "FAIL npm-check"; tail -5 /tmp/zzh-npmcheck.log | sed 's/^/  /'; fi

# Hidden unit tests (copied in, then removed).
if [ -d src/core ]; then
  cp "$C/zzh_hidden.test.ts" src/core/zzh_hidden.test.ts
  timeout 600 npx vitest run src/core/zzh_hidden.test.ts --reporter=json --outputFile=/tmp/zzh-vitest.json >/dev/null 2>&1
  rm -f src/core/zzh_hidden.test.ts
  python3 - <<'PY'
import json
try:
    d = json.load(open("/tmp/zzh-vitest.json"))
    for f in d.get("testResults", []):
        for a in f.get("assertionResults", []):
            print(("PASS " if a["status"] == "passed" else "FAIL ") + a["title"] + " " + " ".join(a.get("failureMessages", []))[:200].replace("\n", " "))
except Exception as e:
    print("FAIL u-crash", e)
PY
  rm -f /tmp/zzh-vitest.json
fi

# Build artifacts (from npm run check's build).
python3 - <<'PY'
import json, os, re
def res(ok, item, d=""): print(("PASS " if ok else "FAIL ") + item + (" " + d if d else ""))
sw = open("dist/sw.js").read() if os.path.isfile("dist/sw.js") else ""
res(bool(sw), "b-sw")
try:
    m = json.load(open("dist/manifest.webmanifest"))
    res(m.get("lang") == "pt-BR" and m.get("display") == "standalone" and m.get("name") == "Caça-Palavras" and
        any(i.get("src", "").endswith("icon.svg") for i in m.get("icons", [])), "b-manifest")
except Exception as e:
    res(False, "b-manifest", str(e)[:100])
need = [r"index\.html", r"\.js", r"\.css", r"\.woff2", r"icon\.svg"]
res(bool(sw) and all(re.search(p, sw) for p in need), "b-precache")
PY

# Browser tests against the production build.
PW=/tmp/qh-pw/node_modules/playwright-core
if [ ! -d "$PW" ]; then mkdir -p /tmp/qh-pw && (cd /tmp/qh-pw && timeout 300 npm install --silent playwright-core@1.56.1 >/dev/null 2>&1); fi
CHROME=$(ls /opt/pw-browsers/chromium-*/chrome-linux/chrome 2>/dev/null | head -1)
if [ -d dist ] && [ -n "$CHROME" ]; then
  port=$((4300 + RANDOM % 500))
  npx vite preview --host 127.0.0.1 --port $port --strictPort >/tmp/zzh-preview.log 2>&1 &
  pid=$!
  for _ in $(seq 1 30); do curl -sf "http://127.0.0.1:$port/" >/dev/null && break; sleep 1; done
  timeout 300 node "$C/e2e.cjs" "$PW" "$CHROME" "http://127.0.0.1:$port"
  kill $pid 2>/dev/null; wait $pid 2>/dev/null
else
  echo "FAIL e2e-setup no dist or chrome"
fi
} > "$log" 2>&1
cat "$log"
python3 "$C/score.py" "$log"
rm -f "$log"
