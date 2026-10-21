#!/usr/bin/env python3
"""A/B contrast between two results dirs on their common tasks.
Usage: eval/runner/compare.py <base-dir> <cand-dir> [--tasks a,b,c]
Prints per-task and aggregate deltas. Uses result.json metadata only (success/score are
for the composer's decision; the proposer works from digests)."""
import json, os, sys, glob, statistics as st

def load(d):
    out = {}
    for f in glob.glob(os.path.join(d, "*", "r*", "result.json")):
        r = json.load(open(f))
        out.setdefault(r["task_id"], []).append(r)
    return out

args = [a for a in sys.argv[1:] if not a.startswith("--")]
only = None
if "--tasks" in sys.argv:
    only = set(sys.argv[sys.argv.index("--tasks") + 1].split(","))
    args = [a for a in args if a != sys.argv[sys.argv.index("--tasks") + 1]]
A, B = load(args[0]), load(args[1])
tasks = sorted(set(A) & set(B))
if only:
    tasks = [t for t in tasks if t in only]

def m(rs, f):
    return st.mean(f(r) for r in rs)

fields = [
    ("succ", lambda r: 1.0 if r["success"] else 0.0, "{:.2f}"),
    ("score", lambda r: r["score"], "{:.2f}"),
    ("turns", lambda r: r["outcome"]["Turns"], "{:.0f}"),
    ("out_tok", lambda r: r["outcome"]["CompletionTokens"], "{:.0f}"),
    ("think", lambda r: r["outcome"]["ReasoningTokens"], "{:.0f}"),
    ("tool_err", lambda r: r["outcome"]["ToolErrors"], "{:.0f}"),
    ("wall_min", lambda r: r["outcome"]["WallSeconds"] / 60, "{:.1f}"),
]
hdr = "| task | n(A/B) | " + " | ".join(f"{n} A→B" for n, _, _ in fields) + " |"
print(hdr); print("|" + "---|" * (2 + len(fields)))
tot = {n: [0, 0] for n, _, _ in fields}
for t in tasks:
    row = [t, f"{len(A[t])}/{len(B[t])}"]
    for n, f, fmt in fields:
        a, b = m(A[t], f), m(B[t], f)
        tot[n][0] += a; tot[n][1] += b
        row.append(f"{fmt.format(a)}→{fmt.format(b)}")
    print("| " + " | ".join(row) + " |")
row = ["**sum**", ""]
for n, f, fmt in fields:
    row.append(f"{fmt.format(tot[n][0])}→{fmt.format(tot[n][1])}")
print("| " + " | ".join(row) + " |")
