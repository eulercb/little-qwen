#!/usr/bin/env python3
"""Aggregate trace.jsonl files under a results dir: where does time go, which tools fail.
Usage: eval/runner/analyze.py eval/results/<iter> [more dirs...]
Reads only traces (never hidden-check output)."""
import json, sys, os, collections

def load(d):
    for root, _, files in os.walk(d):
        if "trace.jsonl" in files:
            yield root, [json.loads(l) for l in open(os.path.join(root, "trace.jsonl"))]

for d in sys.argv[1:]:
    turns = tools = 0
    lat = decode = prefill = comp = think = 0
    tool_err = collections.Counter(); tool_n = collections.Counter(); events = collections.Counter()
    write_tokens = 0; finish_len = collections.Counter()
    for run, recs in load(d):
        for r in recs:
            if r["type"] == "turn":
                turns += 1; lat += r["latency_ms"] / 1000
                comp += r["completion_tokens"]; think += r["reasoning_tokens"]; prefill += r["prefill_tokens"]
                finish_len[r["finish_reason"]] += 1
            elif r["type"] == "tool":
                tools += 1; tool_n[r["tool"]] += 1
                if not r["ok"]:
                    tool_err[(r["tool"], r.get("error_class", ""))] += 1
            elif r["type"] == "event":
                events[r["name"]] += 1
    print(f"== {d}")
    if not turns:
        print("no traces"); continue
    print(f"turns={turns} model_time={lat/60:.1f}min avg_turn={lat/turns:.1f}s completion={comp} (thinking {think}, {100*think/max(comp,1):.0f}%) prefill={prefill}")
    print(f"finish reasons: {dict(finish_len)}")
    print("tool calls:", dict(tool_n.most_common()))
    print("tool errors:", {f"{k[0]}:{k[1]}": v for k, v in tool_err.most_common()})
    print("events:", dict(events.most_common()))
