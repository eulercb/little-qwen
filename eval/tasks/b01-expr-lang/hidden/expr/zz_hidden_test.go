package expr

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestHiddenArithmetic(t *testing.T) {
	cases := map[string]float64{
		"1 + 2 * 3":     7,
		"(1 + 2) * 3":   9,
		"10 - 4 - 3":    3,
		"2 ^ 3 ^ 2":     512,
		"-2 ^ 2":        -4,
		"2 ^ -1":        0.5,
		"7 % 4":         3,
		"-7 % 4":        -3,
		".5 + 1. + 1e3": 1001.5,
		"2.5E-2 * 40":   1,
		"--3":           3,
		"+4":            4,
		"8 / 4 / 2":     1,
		"1 + -2":        -1,
		"2 * 3 ^ 2":     18,
	}
	for src, want := range cases {
		got, err := Eval(src, nil)
		if err != nil || math.Abs(got-want) > 1e-9 {
			t.Errorf("%q = %v, %v; want %v", src, got, err, want)
		}
	}
}

func TestHiddenLogic(t *testing.T) {
	vars := map[string]float64{"x": 5, "y": 0}
	cases := map[string]float64{
		"x > 3 && y == 0":             1,
		"x < 3 || y":                  0,
		"!y":                          1,
		"!x":                          0,
		"x >= 5 == 1":                 1,
		"1 < 2 == 2 > 1":              1,
		"x ? 10 : 20":                 10,
		"y ? 10 : y ? 1 : 2":          2,
		"true + true":                 2,
		"false || 3":                  1,
		"x != 5 ? 1 : x == 5 ? 7 : 9": 7,
	}
	for src, want := range cases {
		got, err := Eval(src, vars)
		if err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", src, got, err, want)
		}
	}
}

func TestHiddenShortCircuit(t *testing.T) {
	for _, src := range []string{"0 && 1/0", "1 || missing", "1 ? 2 : 1/0", "0 ? nope : 3", "0 && sqrt(-1)"} {
		if _, err := Eval(src, nil); err != nil {
			t.Errorf("%q: short-circuit failed: %v", src, err)
		}
	}
}

func TestHiddenFunctions(t *testing.T) {
	cases := map[string]float64{
		"abs(-3)":               3,
		"sqrt(16)":              4,
		"min(3, 1, 2)":          1,
		"max(3)":                3,
		"round(2.5)":            3,
		"clamp(15, 0, 10)":      10,
		"clamp(-1, 0, 10)":      0,
		"max(1, min(5, 4)) * 2": 8,
	}
	for src, want := range cases {
		got, err := Eval(src, nil)
		if err != nil || got != want {
			t.Errorf("%q = %v, %v; want %v", src, got, err, want)
		}
	}
	got, err := Eval("abs + 1", map[string]float64{"abs": 2})
	if err != nil || got != 3 {
		t.Errorf("abs as variable: %v %v", got, err)
	}
}

func wantErr(t *testing.T, err error, pos int, msg string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Errorf("want *Error{%d,%q}, got %T %v", pos, msg, err, err)
		return
	}
	if e.Pos != pos || e.Msg != msg {
		t.Errorf("got {%d,%q} want {%d,%q}", e.Pos, e.Msg, pos, msg)
	}
	if e.Error() != errorString(pos, msg) {
		t.Errorf("Error() = %q", e.Error())
	}
}

func errorString(pos int, msg string) string { return itoa(pos) + ": " + msg }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func TestHiddenCompileErrors(t *testing.T) {
	_, err := Compile("1 + $")
	wantErr(t, err, 4, "unexpected character '$'")
	_, err = Compile("(1 + 2")
	wantErr(t, err, 6, `expected ")"`)
	_, err = Compile("1 ? 2 3")
	wantErr(t, err, 6, `expected ":"`)
	_, err = Compile("   ")
	wantErr(t, err, 3, "unexpected end of input")
	_, err = Compile("1 2")
	wantErr(t, err, 2, `unexpected token "2"`)
	_, err = Compile("1 + * 2")
	wantErr(t, err, 4, `unexpected token "*"`)
	_, err = Compile("foo(1)")
	wantErr(t, err, 0, `unknown function "foo"`)
	_, err = Compile("1 + clamp(1, 2)")
	wantErr(t, err, 4, `function "clamp" expects 3 arguments, got 2`)
	_, err = Compile("min()")
	wantErr(t, err, 0, `function "min" expects at least 1 argument, got 0`)
	_, err = Compile("2 *")
	wantErr(t, err, 3, "unexpected end of input")
}

func TestHiddenRuntimeErrors(t *testing.T) {
	_, err := Eval("1 + 2 / (x - x)", map[string]float64{"x": 1})
	wantErr(t, err, 6, "division by zero")
	_, err = Eval("5 % 0", nil)
	wantErr(t, err, 2, "division by zero")
	_, err = Eval("a + bee", map[string]float64{"a": 1})
	wantErr(t, err, 4, `unknown variable "bee"`)
	_, err = Eval("1 + sqrt(-4)", nil)
	wantErr(t, err, 4, "sqrt of negative number")
	_, err = Eval("10 ^ 400", nil)
	wantErr(t, err, 0, "result is not finite")
}

func TestHiddenProgramReuseAndVars(t *testing.T) {
	p, err := Compile("b * a + max(a, c) - b")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Vars(), []string{"a", "b", "c"}) {
		t.Fatalf("Vars = %v", p.Vars())
	}
	for i := 1; i <= 3; i++ {
		f := float64(i)
		got, err := p.Eval(map[string]float64{"a": f, "b": 2, "c": 0})
		if err != nil || got != 2*f+f-2 {
			t.Fatalf("i=%d got %v %v", i, got, err)
		}
	}
	q, _ := Compile("true ? 1 : 2")
	if len(q.Vars()) != 0 {
		t.Fatalf("literals are not vars: %v", q.Vars())
	}
}
