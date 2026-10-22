#!/usr/bin/env python3
"""Static checks of the brief's hard rules. Prints `PASS|FAIL <item> <detail>`."""
import json, os, re, sys

task_dir = sys.argv[1]
brief = open(os.path.join(task_dir, "prompt.md"), encoding="utf-8").read()

def res(ok, item, detail=""):
    print(("PASS " if ok else "FAIL ") + item + (" " + detail if detail else ""))

def appendix(title):
    i = brief.index(title); j = brief.index("```ts", i) + len("```ts\n"); k = brief.index("```", j)
    return brief[j:k]

def norm(s):
    return "\n".join(l.rstrip() for l in s.strip().splitlines())

required = """package.json tsconfig.json vite.config.ts vitest.config.ts index.html public/icons/icon.svg src/main.ts
src/style.css src/core/types.ts src/core/normalize.ts src/core/prng.ts src/core/directions.ts src/core/levels.ts
src/core/generate.ts src/core/select.ts src/core/save.ts src/data/words.ts src/data/letters.ts src/data/blocklist.ts
src/ui/strings.ts src/ui/router.ts src/ui/storage.ts src/ui/home.ts src/ui/puzzle.ts src/ui/grid.ts src/ui/pointer.ts
src/ui/settings.ts src/core/normalize.test.ts src/core/prng.test.ts src/core/directions.test.ts src/core/levels.test.ts
src/data/words.test.ts src/core/generate.test.ts src/core/select.test.ts src/core/save.test.ts""".split()
missing = [f for f in required if not os.path.isfile(f)]
res(not missing, "layout", ",".join(missing[:8]))

src_files = []
for root, _, files in os.walk("src"):
    for f in files:
        if f.endswith((".ts", ".css")) and "zzh_" not in f:
            src_files.append(os.path.join(root, f))
long = [f"{f}:{sum(1 for _ in open(f, encoding='utf-8'))}" for f in src_files if sum(1 for _ in open(f, encoding="utf-8")) > 250]
res(bool(src_files) and not long, "file-length", ",".join(long))

bad = []
for f in src_files:
    if not f.endswith(".ts"):
        continue
    text = open(f, encoding="utf-8").read()
    for pat, why in [(r"\bTODO\b", "TODO"), (r"@ts-ignore|@ts-nocheck", "ts-ignore"), (r"\bconsole\.log\b", "console.log"),
                     (r"(:\s*any\b|\bas\s+any\b|<any>|\bany\[\])", "any")]:
        if re.search(pat, text):
            bad.append(f"{f}:{why}")
res(bool(src_files) and not bad, "hygiene", ",".join(bad[:8]))

core_dom = []
for f in src_files:
    if f.startswith(os.path.join("src", "core")) and f.endswith(".ts") and not f.endswith(".test.ts"):
        code = re.sub(r"//.*|/\*[\s\S]*?\*/", "", open(f, encoding="utf-8").read())
        if re.search(r"\b(document|window|navigator|localStorage)\b", code):
            core_dom.append(f)
res(bool(src_files) and not core_dom, "core-pure", ",".join(core_dom))

for name, path, title in [("data-words", "src/data/words.ts", "## Appendix A"), ("data-letters", "src/data/letters.ts", "## Appendix B"),
                          ("strings", "src/ui/strings.ts", "## Appendix C")]:
    ok = os.path.isfile(path) and norm(open(path, encoding="utf-8").read()) == norm(appendix(title))
    res(ok, name)

ok, detail = False, ""
try:
    pkg = json.load(open("package.json"))
    deps = set(pkg.get("dependencies", {})) | set(pkg.get("devDependencies", {}))
    allowed = {"vite", "typescript", "vitest", "vite-plugin-pwa", "@fontsource/nunito"}
    extra = deps - allowed
    ok = not extra and {"vite", "typescript", "vitest", "vite-plugin-pwa", "@fontsource/nunito"} <= deps \
        and pkg.get("scripts", {}).get("check") == "npm run typecheck && npm run test && npm run build"
    detail = ",".join(sorted(extra))
except Exception as e:
    detail = str(e)
res(ok, "dependencies", detail)

idx = open("index.html", encoding="utf-8").read() if os.path.isfile("index.html") else ""
res('lang="pt-BR"' in idx and "theme-color" in idx and "viewport-fit=cover" in idx, "index-html")
