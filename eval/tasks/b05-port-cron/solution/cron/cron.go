package cron

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

type field struct {
	name   string
	lo, hi int
}

var fields = []field{{"minute", 0, 59}, {"hour", 0, 23}, {"day-of-month", 1, 31}, {"month", 1, 12}, {"day-of-week", 0, 6}}
var months = []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
var days = []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}
var macros = map[string]string{"@yearly": "0 0 1 1 *", "@annually": "0 0 1 1 *", "@monthly": "0 0 1 * *",
	"@weekly": "0 0 * * 0", "@daily": "0 0 * * *", "@midnight": "0 0 * * *", "@hourly": "0 * * * *"}

type Schedule struct {
	expr             string
	sets             [5]map[int]bool
	domStar, dowStar bool
}

// pyRepr mimics Python's repr() for simple strings.
func pyRepr(s string) string {
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		return `"` + s + `"`
	}
	return "'" + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "'", `\'`) + "'"
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func value(tok string, idx int) (int, error) {
	f := fields[idx]
	t := strings.ToUpper(tok)
	if idx == 3 {
		for i, m := range months {
			if t == m {
				return i + 1, nil
			}
		}
	}
	if idx == 4 {
		for i, d := range days {
			if t == d {
				return i, nil
			}
		}
	}
	if !isDigits(tok) {
		return 0, fmt.Errorf("%s: invalid value %s", f.name, pyRepr(tok))
	}
	v, _ := strconv.Atoi(tok)
	if idx == 4 && v == 7 {
		v = 0
	}
	if v < f.lo || v > f.hi {
		return 0, fmt.Errorf("%s: value %d out of range %d-%d", f.name, v, f.lo, f.hi)
	}
	return v, nil
}

func parseField(text string, idx int) (map[int]bool, error) {
	f := fields[idx]
	if text == "" {
		return nil, fmt.Errorf("%s: empty field", f.name)
	}
	vals := map[int]bool{}
	for _, item := range strings.Split(text, ",") {
		if item == "" {
			return nil, fmt.Errorf("%s: empty item", f.name)
		}
		step := 1
		base := item
		hasStep := strings.Contains(item, "/")
		if hasStep {
			var s string
			base, s, _ = strings.Cut(item, "/")
			if !isDigits(s) || atoi(s) < 1 {
				return nil, fmt.Errorf("%s: invalid step %s", f.name, pyRepr(s))
			}
			step = atoi(s)
		}
		var start, end int
		switch {
		case base == "*":
			start, end = f.lo, f.hi
		case strings.Contains(base, "-"):
			a, b, _ := strings.Cut(base, "-")
			var err error
			if start, err = value(a, idx); err != nil {
				return nil, err
			}
			if end, err = value(b, idx); err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("%s: range %d-%d is reversed", f.name, start, end)
			}
		default:
			var err error
			if start, err = value(base, idx); err != nil {
				return nil, err
			}
			end = start
			if hasStep {
				end = f.hi
			}
		}
		for v := start; v <= end; v += step {
			vals[v] = true
		}
	}
	return vals, nil
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 1 << 30
	}
	return n
}

func Parse(expr string) (*Schedule, error) {
	original := strings.TrimSpace(expr)
	e := original
	if strings.HasPrefix(e, "@") {
		m, ok := macros[strings.ToLower(e)]
		if !ok {
			return nil, fmt.Errorf("unknown macro %s", pyRepr(e))
		}
		e = m
	}
	parts := strings.Fields(e)
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}
	s := &Schedule{expr: original, domStar: parts[2] == "*", dowStar: parts[4] == "*"}
	for i, p := range parts {
		set, err := parseField(p, i)
		if err != nil {
			return nil, err
		}
		s.sets[i] = set
	}
	return s, nil
}

func (s *Schedule) dayOK(t time.Time) bool {
	inDom := s.sets[2][t.Day()]
	inDow := s.sets[4][int(t.Weekday())]
	if !s.domStar && !s.dowStar {
		return inDom || inDow
	}
	return inDom && inDow
}

func (s *Schedule) Next(after time.Time) (time.Time, error) {
	loc := after.Location()
	t := time.Date(after.Year(), after.Month(), after.Day(), after.Hour(), after.Minute(), 0, 0, loc).Add(time.Minute)
	limit := after.Add(366 * 5 * 24 * time.Hour)
	for !t.After(limit) {
		if !s.sets[3][int(t.Month())] {
			y, m := t.Year(), t.Month()+1
			if t.Month() == 12 {
				y, m = y+1, 1
			}
			t = time.Date(y, m, 1, 0, 0, 0, 0, loc)
			continue
		}
		if !s.dayOK(t) {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
			continue
		}
		if !s.sets[1][t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc).Add(time.Hour)
			continue
		}
		if !s.sets[0][t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}
		return t, nil
	}
	return time.Time{}, errors.New("no match within 5 years for " + pyRepr(s.expr))
}

func format(vals map[int]bool, idx int) string {
	f := fields[idx]
	full := len(vals) == f.hi-f.lo+1
	if full {
		for v := f.lo; v <= f.hi; v++ {
			if !vals[v] {
				full = false
			}
		}
	}
	if full {
		return "*"
	}
	var vs []int
	for v := range vals {
		vs = append(vs, v)
	}
	sort.Ints(vs)
	var out []string
	for i := 0; i < len(vs); {
		j := i
		for j+1 < len(vs) && vs[j+1] == vs[j]+1 {
			j++
		}
		if j-i < 2 {
			out = append(out, strconv.Itoa(vs[i]))
		} else {
			out = append(out, fmt.Sprintf("%d-%d", vs[i], vs[j]))
		}
		if j-i > 0 && j-i < 2 {
			for _, v := range vs[i+1 : j+1] {
				out = append(out, strconv.Itoa(v))
			}
		}
		i = j + 1
	}
	return strings.Join(out, ",")
}

func (s *Schedule) String() string {
	var parts []string
	for i := range s.sets {
		parts = append(parts, format(s.sets[i], i))
	}
	return strings.Join(parts, " ")
}
