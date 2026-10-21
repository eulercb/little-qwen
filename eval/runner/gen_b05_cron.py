# Generates eval/tasks/b05-port-cron/hidden/cron/zz_hidden_test.go from the Python reference.
import sys, json, importlib.util
from datetime import datetime, timezone, timedelta
spec = importlib.util.spec_from_file_location("cron", sys.argv[1]); cron = importlib.util.module_from_spec(spec); spec.loader.exec_module(cron)
exprs = ["* * * * *", "*/15 9-17 * * MON-FRI", "0 0 1 1 *", "@daily", "@WEEKLY", "@hourly", "@yearly", "@monthly", "@midnight",
 "5,6,7,9 1 1-3,5 jan-mar,dec 7", "0 12 13 * 5", "30 2 29 2 *", "0 0 31 * *", "0 0 * * 7", "10-50/20 */6 * * *",
 "15/10 3/7 2/10 4/3 1/2", "0 0 1,15 * 1", "59 23 31 12 *", "0 9 * * sun,sat", "*/7 * * * *", "0 0 1-7 * MON",
 "  1 2 3 4 5  ", "0 0 * jun-aug wed", "1,2,3,4,5,6 * * * *", "0 0 L * *", "60 * * * *", "* 24 * * *", "* * 0 * *",
 "* * * 13 *", "* * * * 8", "5-1 * * * *", "* * * * 5-7", "*/0 * * * *", "*/ * * * *", "1,,2 * * * *", "* * * *",
 "* * * * * *", "@reboot", "@daily extra", "a * * * *", "* * * FOO *", "1-2-3 * * * *", "*/x * * * *", "", "0 0 31 2 *",
 "-1 * * * *", "1- * * * *", "0 0 30 2 *", "0 0 29 2 1", "3-3 * * * *", "0-59/30 0-23/12 * * *"]
starts = [datetime(2026,10,21,16,50,tzinfo=timezone.utc), datetime(2028,2,28,23,59,30,tzinfo=timezone.utc),
          datetime(2026,12,31,23,59,tzinfo=timezone(timedelta(hours=5,minutes=30)))]
cases = []
for e in exprs:
    try:
        s = cron.parse(e)
    except cron.CronError as err:
        cases.append({"expr": e, "err": str(err)}); continue
    runs = []
    for st in starts:
        t, seq = st, []
        try:
            for _ in range(4):
                t = cron.next_run(s, t); seq.append(t.isoformat())
            runs.append({"start": st.isoformat(), "next": seq})
        except cron.CronError as err:
            runs.append({"start": st.isoformat(), "next": seq, "err": str(err)})
    cases.append({"expr": e, "describe": cron.describe(s), "runs": runs})
print(f'''package cron

import (
	"encoding/json"
	"testing"
	"time"
)

// Golden data generated from reference/cron.py by eval/runner/gen_b05_cron.py.
const zzhGolden = {json.dumps(json.dumps(cases))}

type zzhRun struct {{
	Start string   `json:"start"`
	Next  []string `json:"next"`
	Err   string   `json:"err"`
}}

type zzhCase struct {{
	Expr     string   `json:"expr"`
	Err      string   `json:"err"`
	Describe string   `json:"describe"`
	Runs     []zzhRun `json:"runs"`
}}

func zzhCases(t *testing.T) []zzhCase {{
	var cs []zzhCase
	if err := json.Unmarshal([]byte(zzhGolden), &cs); err != nil {{
		t.Fatal(err)
	}}
	return cs
}}

func TestHiddenParseErrors(t *testing.T) {{
	for _, c := range zzhCases(t) {{
		s, err := Parse(c.Expr)
		if c.Err != "" {{
			if err == nil || err.Error() != c.Err {{
				t.Errorf("Parse(%q) error = %v, want %q", c.Expr, err, c.Err)
			}}
			continue
		}}
		if err != nil {{
			t.Errorf("Parse(%q) unexpected error %v", c.Expr, err)
			continue
		}}
		_ = s
	}}
}}

func TestHiddenDescribe(t *testing.T) {{
	for _, c := range zzhCases(t) {{
		if c.Err != "" {{
			continue
		}}
		s, err := Parse(c.Expr)
		if err != nil {{
			continue
		}}
		if got := s.String(); got != c.Describe {{
			t.Errorf("String(%q) = %q, want %q", c.Expr, got, c.Describe)
		}}
	}}
}}

func TestHiddenNext(t *testing.T) {{
	for _, c := range zzhCases(t) {{
		if c.Err != "" {{
			continue
		}}
		s, err := Parse(c.Expr)
		if err != nil {{
			continue
		}}
		for _, r := range c.Runs {{
			cur, _ := time.Parse(time.RFC3339, r.Start)
			if _, off := cur.Zone(); off != 0 {{
				cur = cur.In(time.FixedZone("", off))
			}} else {{
				cur = cur.UTC()
			}}
			for i, want := range r.Next {{
				got, err := s.Next(cur)
				if err != nil {{
					t.Errorf("%q from %s step %d: %v", c.Expr, r.Start, i, err)
					break
				}}
				if got.Format("2006-01-02T15:04:05-07:00") != zzhPyISO(want) {{
					t.Errorf("%q from %s step %d: got %s want %s", c.Expr, r.Start, i, got.Format(time.RFC3339), want)
					break
				}}
				cur = got
			}}
			if r.Err != "" {{
				_, err := s.Next(cur)
				if err == nil || err.Error() != r.Err {{
					t.Errorf("%q from %s: error %v, want %q", c.Expr, r.Start, err, r.Err)
				}}
			}}
		}}
	}}
}}

// zzhPyISO converts Python's isoformat (+00:00) to Go's layout.
func zzhPyISO(s string) string {{ return s }}
''')
