#!/usr/bin/env python3
"""Reference cron-expression library.

Syntax: five space-separated fields: minute hour day-of-month month day-of-week
  minute 0-59, hour 0-23, day-of-month 1-31, month 1-12 (or JAN-DEC), day-of-week 0-6 (or SUN-SAT; 7 is also Sunday)
Each field is a comma-separated list of items. An item is:
  *            every value
  N            a single value
  N-M          an inclusive range (N <= M required)
  */S  N-M/S   a step over the full range or over a range (S >= 1)
  N/S          from N to the field maximum with step S
Names (JAN..DEC, SUN..SAT) are case-insensitive and may be used wherever a number may.
Macros: @yearly (= "0 0 1 1 *"), @monthly ("0 0 1 * *"), @weekly ("0 0 * * 0"), @daily ("0 0 * * *"), @hourly ("0 * * * *").
Day matching follows Vixie cron: if both day-of-month and day-of-week are restricted (neither field is "*"),
a day matches when EITHER matches; otherwise both must match.
"""
import sys
from datetime import datetime, timedelta, timezone

class CronError(ValueError):
    pass

FIELDS = [("minute", 0, 59), ("hour", 0, 23), ("day-of-month", 1, 31), ("month", 1, 12), ("day-of-week", 0, 6)]
MONTHS = ["JAN","FEB","MAR","APR","MAY","JUN","JUL","AUG","SEP","OCT","NOV","DEC"]
DAYS = ["SUN","MON","TUE","WED","THU","FRI","SAT"]
MACROS = {"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
          "@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *"}

def _value(tok, idx):
    name, lo, hi = FIELDS[idx]
    t = tok.upper()
    if idx == 3 and t in MONTHS:
        return MONTHS.index(t) + 1
    if idx == 4 and t in DAYS:
        return DAYS.index(t)
    if not tok.isdigit():
        raise CronError(f"{name}: invalid value {tok!r}")
    v = int(tok)
    if idx == 4 and v == 7:
        v = 0
    if v < lo or v > hi:
        raise CronError(f"{name}: value {v} out of range {lo}-{hi}")
    return v

def _field(text, idx):
    name, lo, hi = FIELDS[idx]
    if text == "":
        raise CronError(f"{name}: empty field")
    values = set()
    for item in text.split(","):
        if item == "":
            raise CronError(f"{name}: empty item")
        step = 1
        if "/" in item:
            base, _, s = item.partition("/")
            if not s.isdigit() or int(s) < 1:
                raise CronError(f"{name}: invalid step {s!r}")
            step = int(s)
        else:
            base = item
        if base == "*":
            start, end = lo, hi
        elif "-" in base:
            a, _, b = base.partition("-")
            start, end = _value(a, idx), _value(b, idx)
            if start > end:
                raise CronError(f"{name}: range {start}-{end} is reversed")
        else:
            start = _value(base, idx)
            end = hi if "/" in item else start
        values.update(range(start, end + 1, step))
    return frozenset(values)

class Schedule:
    def __init__(self, expr, fields, dom_star, dow_star):
        self.expr = expr
        self.minute, self.hour, self.dom, self.month, self.dow = fields
        self.dom_star, self.dow_star = dom_star, dow_star

def parse(expr):
    original = expr
    expr = expr.strip()
    if expr.startswith("@"):
        if expr.lower() not in MACROS:
            raise CronError(f"unknown macro {expr!r}")
        expr = MACROS[expr.lower()]
    parts = expr.split()
    if len(parts) != 5:
        raise CronError(f"expected 5 fields, got {len(parts)}")
    fields = [_field(p, i) for i, p in enumerate(parts)]
    return Schedule(original.strip(), fields, parts[2] == "*", parts[4] == "*")

def _day_ok(s, d):
    dow = (d.weekday() + 1) % 7
    in_dom = d.day in s.dom
    in_dow = dow in s.dow
    if not s.dom_star and not s.dow_star:
        return in_dom or in_dow
    return in_dom and in_dow

def next_run(s, after):
    t = after.replace(second=0, microsecond=0) + timedelta(minutes=1)
    limit = after + timedelta(days=366 * 5)
    while t <= limit:
        if t.month not in s.month:
            # jump to the first minute of the next month
            y, m = (t.year + 1, 1) if t.month == 12 else (t.year, t.month + 1)
            t = t.replace(year=y, month=m, day=1, hour=0, minute=0)
            continue
        if not _day_ok(s, t):
            t = t.replace(hour=0, minute=0) + timedelta(days=1)
            continue
        if t.hour not in s.hour:
            t = t.replace(minute=0) + timedelta(hours=1)
            continue
        if t.minute not in s.minute:
            t = t + timedelta(minutes=1)
            continue
        return t
    raise CronError(f"no match within 5 years for {s.expr!r}")

def _fmt(values, idx):
    name, lo, hi = FIELDS[idx]
    if set(values) == set(range(lo, hi + 1)):
        return "*"
    vs = sorted(values)
    out, i = [], 0
    while i < len(vs):
        j = i
        while j + 1 < len(vs) and vs[j + 1] == vs[j] + 1:
            j += 1
        out.append(str(vs[i]) if j - i < 2 else f"{vs[i]}-{vs[j]}")
        if 0 < j - i < 2:
            out.extend(str(v) for v in vs[i + 1:j + 1])
        i = j + 1
    return ",".join(out)

def describe(s):
    f = [s.minute, s.hour, s.dom, s.month, s.dow]
    return " ".join(_fmt(v, i) for i, v in enumerate(f))

def main(argv):
    if len(argv) < 2 or argv[1] in ("-h", "--help"):
        print("usage: cron.py EXPR [ISO-TIME [N]]   -- prints describe() and the next N runs after ISO-TIME (default 3)")
        return 0
    try:
        s = parse(argv[1])
    except CronError as e:
        print(f"error: {e}")
        return 1
    print(describe(s))
    if len(argv) > 2:
        t = datetime.fromisoformat(argv[2])
        if t.tzinfo is None:
            t = t.replace(tzinfo=timezone.utc)
        for _ in range(int(argv[3]) if len(argv) > 3 else 3):
            try:
                t = next_run(s, t)
            except CronError as e:
                print(f"error: {e}")
                return 1
            print(t.isoformat())
    return 0

if __name__ == "__main__":
    sys.exit(main(sys.argv))
