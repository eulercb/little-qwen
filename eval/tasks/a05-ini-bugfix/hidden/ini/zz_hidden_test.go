package ini

import (
	"errors"
	"strings"
	"testing"
)

func zzhMustGet(t *testing.T, f File, sec, key, want string) {
	t.Helper()
	got, ok := f.Get(sec, key)
	if !ok || got != want {
		t.Errorf("%s.%s = %q,%v want %q", sec, key, got, ok, want)
	}
}

func TestHiddenHashComments(t *testing.T) {
	f, err := Parse(strings.NewReader("# top\n  # indented\n[a]\nk = v # trailing\nc = x#notcomment\n"))
	if err != nil {
		t.Fatal(err)
	}
	zzhMustGet(t, f, "a", "k", "v")
	zzhMustGet(t, f, "a", "c", "x#notcomment")
}

func TestHiddenSectionCase(t *testing.T) {
	f, err := Parse(strings.NewReader("[  MiXeD  ]\nKey = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	zzhMustGet(t, f, "mixed", "key", "1")
	if _, ok := f["mixed"]; !ok {
		t.Fatalf("section map keys: %v", f)
	}
}

func TestHiddenFirstEquals(t *testing.T) {
	f, _ := Parse(strings.NewReader("expr = a=b=c\n"))
	zzhMustGet(t, f, "", "expr", "a=b=c")
}

func TestHiddenOverwrite(t *testing.T) {
	f, _ := Parse(strings.NewReader("[s]\nk = 1\nk = 2\n"))
	zzhMustGet(t, f, "s", "k", "2")
}

func TestHiddenQuotedKeepsSpaceAndSemicolon(t *testing.T) {
	f, _ := Parse(strings.NewReader(`v = "  a ; b  "` + "\n"))
	zzhMustGet(t, f, "", "v", "  a ; b  ")
}

func TestHiddenSemicolonInsideValue(t *testing.T) {
	f, _ := Parse(strings.NewReader("dsn = user;pass\n"))
	zzhMustGet(t, f, "", "dsn", "user;pass")
}

func TestHiddenSyntaxLine(t *testing.T) {
	_, err := Parse(strings.NewReader("\n\n[ok]\n  nope\n"))
	var se *SyntaxError
	if !errors.As(err, &se) || se.Line != 4 {
		t.Fatalf("err = %v", err)
	}
}
