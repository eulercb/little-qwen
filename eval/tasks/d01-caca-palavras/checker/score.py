#!/usr/bin/env python3
"""Aggregate PASS/FAIL lines against the fixed item list; print SCORE p n."""
import sys
ITEMS = """layout file-length hygiene core-pure data-words data-letters strings dependencies index-html npm-check
u-normalize u-prng-exact u-shuffle-exact u-directions u-levels u-select-cases u-select-hysteresis u-matchword u-save
u-generate-valid u-generate-deterministic u-generate-substring u-generate-blocklist u-letterat
b-sw b-manifest b-precache
e-home e-puzzle-tier1 e-puzzle-tier4 e-drag-forward e-drag-reverse e-miss e-hint e-persist e-complete e-settings-dark
e-offline e-trace-hint""".split()
status = {}
for line in open(sys.argv[1], encoding="utf-8", errors="replace"):
    parts = line.split()
    if len(parts) >= 2 and parts[0] in ("PASS", "FAIL"):
        status[parts[1]] = parts[0] == "PASS" and status.get(parts[1], True)
passed = 0
for it in ITEMS:
    ok = status.get(it, False)
    passed += ok
    print(f"{'ok  ' if ok else 'MISS'} {it}")
print(f"SCORE {passed} {len(ITEMS)}")
