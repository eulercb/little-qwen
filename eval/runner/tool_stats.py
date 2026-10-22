#!/usr/bin/env python3
"""Tool-use pattern stats over qh transcripts (used to vet Little Coder's
write-guard / read-before-edit ideas against our own data).

  eval/runner/tool_stats.py eval/results/it0-baseline [more result dirs...]

Per run: write_file calls that overwrote an existing file (and how big that
file was when last read), edit_file calls on a file never read in that run,
edit_file failures by error code, and the run's score.
"""
import glob, json, os, re, sys
from collections import Counter

def runs(dirs):
    for d in dirs:
        for t in sorted(glob.glob(os.path.join(d, "*", "r*", "transcript.jsonl"))):
            yield os.path.dirname(t)

def score(rdir):
    try:
        r = json.load(open(os.path.join(rdir, "result.json")))
        return r.get("score")
    except Exception:
        return None

tot = Counter()
errs = Counter()
rows = []
for rdir in runs(sys.argv[1:]):
    calls = {}
    read_lines = {}   # path -> total line count seen by read_file
    seen = set()      # paths read or written in this run
    c = Counter()
    for l in open(os.path.join(rdir, "transcript.jsonl")):
        d = json.loads(l)
        if d.get("agent", "main") != "main":
            continue
        m = d["msg"]
        if m["role"] == "assistant":
            for tc in m.get("tool_calls") or []:
                try:
                    a = json.loads(tc["function"]["arguments"])
                except Exception:
                    a = {}
                calls[tc["id"]] = (tc["function"]["name"], a)
                name, p = tc["function"]["name"], a.get("path", "")
                if name == "edit_file":
                    c["edit"] += 1
                    if p not in seen:
                        c["edit_unread"] += 1
                if name == "write_file":
                    c["write"] += 1
        elif m["role"] == "tool":
            name, a = calls.get(m.get("tool_call_id"), (m.get("name"), {}))
            out = m.get("content") or ""
            p = a.get("path", "")
            if name == "read_file":
                mm = re.match(r"OK \S+ lines \d+-\d+ of (\d+)", out)
                if mm:
                    read_lines[p] = int(mm.group(1))
                seen.add(p)
            elif name == "write_file":
                if out.startswith("OK overwrote"):
                    c["overwrite"] += 1
                    if read_lines.get(p, 0) > 40:
                        c["overwrite_big"] += 1
                    if p not in seen:
                        c["overwrite_unread"] += 1
                seen.add(p)
            elif name == "edit_file":
                if out.startswith("OK"):
                    seen.add(p)
                else:
                    code = out.split(":")[0].split()[-1] if out else "?"
                    c["edit_fail"] += 1
                    errs[code[:40]] += 1
    tot.update(c)
    tot["runs"] += 1
    rows.append((os.path.relpath(rdir), score(rdir), c))

for r, s, c in rows:
    print(f"{r:60s} score={s!s:5s} " + " ".join(f"{k}={v}" for k, v in sorted(c.items())))
print("\nTOTAL", dict(tot))
print("edit_file failure codes:", dict(errs.most_common(10)))
