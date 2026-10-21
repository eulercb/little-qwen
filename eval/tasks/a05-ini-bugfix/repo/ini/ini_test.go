package ini

import (
	"errors"
	"strings"
	"testing"
)

const doc = `
; global settings
name = demo
  [Server]
host = example.com
url = http://x/?a=b
  port = 8080   ; inline comment

# another comment
[Paths]
root = "  /srv  "
`

func TestParse(t *testing.T) {
	f, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	check := func(sec, key, want string) {
		t.Helper()
		got, ok := f.Get(sec, key)
		if !ok || got != want {
			t.Errorf("%s.%s = %q,%v want %q", sec, key, got, ok, want)
		}
	}
	check("", "name", "demo")
	check("server", "host", "example.com")
	check("server", "url", "http://x/?a=b")
	check("server", "port", "8080")
	check("paths", "root", "  /srv  ")
}

func TestSyntaxError(t *testing.T) {
	_, err := Parse(strings.NewReader("a = 1\njunk line\n"))
	var se *SyntaxError
	if !errors.As(err, &se) || se.Line != 2 {
		t.Fatalf("err = %v", err)
	}
}
