package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goSrc = "package x\n\nfunc f() {\n\tif a {\n\t\tif b {\n\t\t\treturn 1\n\t\t}\n\t}\n}\n"

func TestFuzzyReplaceReindents(t *testing.T) {
	// Model copied one tab too many (the read_file separator tab).
	old := "\t\t\tif b {\n\t\t\t\treturn 1\n\t\t\t}"
	new := "\t\t\tif b && c {\n\t\t\t\treturn 2\n\t\t\t}"
	got, line, _, ok := fuzzyReplace(goSrc, old, new)
	if !ok || line != 5 {
		t.Fatalf("ok=%v line=%d", ok, line)
	}
	want := "package x\n\nfunc f() {\n\tif a {\n\t\tif b && c {\n\t\t\treturn 2\n\t\t}\n\t}\n}\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestFuzzyReplaceSpacesForTabs(t *testing.T) {
	old := "    if b {\n        return 1\n    }"
	new := "    if b {\n        return 3\n    }"
	got, _, _, ok := fuzzyReplace(goSrc, old, new)
	if !ok || !strings.Contains(got, "\t\tif b {\n\t\t\treturn 3\n\t\t}") {
		t.Fatalf("ok=%v\n%s", ok, got)
	}
}

func TestFuzzyReplaceAmbiguous(t *testing.T) {
	src := "a\n\tx\nb\n\tx\n"
	if _, _, _, ok := fuzzyReplace(src, "  x", "y"); ok {
		t.Fatal("ambiguous match accepted")
	}
}

func TestEditFileFuzzyFlag(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.go")
	os.WriteFile(p, []byte(goSrc), 0o644)
	e := &Env{Root: dir}
	r := e.editFile(Args{"path": "f.go", "old_text": "\t\t\t\treturn 1", "new_text": "\t\t\t\treturn 9"})
	if r.OK {
		t.Fatal("strict mode should fail")
	}
	e.FuzzyEdit = true
	r = e.editFile(Args{"path": "f.go", "old_text": "\t\t\t\treturn 1", "new_text": "\t\t\t\treturn 9"})
	b, _ := os.ReadFile(p)
	if !r.OK || !strings.Contains(string(b), "\n\t\t\treturn 9\n") {
		t.Fatalf("%v\n%s", r.Output, b)
	}
}

func TestReadFileSeparator(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f.go"), []byte(goSrc), 0o644)
	e := &Env{Root: dir, LineSep: "| "}
	r := e.readFile(Args{"path": "f.go"})
	if !strings.Contains(r.Output, "    6| \t\t\treturn 1\n") {
		t.Fatalf("%q", r.Output)
	}
}
