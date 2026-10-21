// Package ini parses a simple INI dialect:
//
//   - Lines are trimmed of surrounding whitespace. Empty lines are ignored.
//   - Lines starting with ';' or '#' are comments.
//   - "[name]" starts a section. Section names are trimmed and lower-cased.
//     Keys before any section belong to the section "" (empty string).
//   - "key = value" sets a key. Keys are trimmed and lower-cased; values are
//     trimmed. The first '=' separates key and value, so values may contain '='.
//   - A value wrapped in double quotes has the quotes removed and keeps its
//     inner whitespace exactly ("  a  " -> "  a  ").
//   - An inline comment starts at " ;" or " #" (whitespace then ; or #) in an
//     unquoted value and runs to the end of the line.
//   - A repeated key in the same section overwrites the earlier value.
//   - Any other line is a syntax error reported as *SyntaxError with the
//     1-based line number.
package ini

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// File maps section -> key -> value.
type File map[string]map[string]string

// SyntaxError reports a malformed line.
type SyntaxError struct {
	Line int
	Text string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("ini: line %d: syntax error: %q", e.Line, e.Text) }

// Get returns the value of key in section.
func (f File) Get(section, key string) (string, bool) {
	s, ok := f[strings.ToLower(section)]
	if !ok {
		return "", false
	}
	v, ok := s[strings.ToLower(key)]
	return v, ok
}

// Parse reads an INI document.
func Parse(r io.Reader) (File, error) {
	f := File{}
	section := ""
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := sc.Text()
		if line == "" || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		i := strings.LastIndex(line, "=")
		if i < 0 {
			return nil, &SyntaxError{Line: n, Text: line}
		}
		key := strings.ToLower(strings.TrimSpace(line[:i]))
		val := strings.TrimSpace(line[i+1:])
		if j := strings.Index(val, ";"); j >= 0 {
			val = strings.TrimSpace(val[:j])
		}
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = strings.TrimSpace(val[1 : len(val)-1])
		}
		if f[section] == nil {
			f[section] = map[string]string{}
		}
		if _, exists := f[section][key]; !exists {
			f[section][key] = val
		}
	}
	return f, sc.Err()
}
