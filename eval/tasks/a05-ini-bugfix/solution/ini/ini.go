// Package ini parses a simple INI dialect (see original doc).
package ini

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type File map[string]map[string]string

type SyntaxError struct {
	Line int
	Text string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("ini: line %d: syntax error: %q", e.Line, e.Text) }

func (f File) Get(section, key string) (string, bool) {
	s, ok := f[strings.ToLower(section)]
	if !ok {
		return "", false
	}
	v, ok := s[strings.ToLower(key)]
	return v, ok
}

func Parse(r io.Reader) (File, error) {
	f := File{}
	section := ""
	sc := bufio.NewScanner(r)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if f[section] == nil {
				f[section] = map[string]string{}
			}
			continue
		}
		i := strings.Index(line, "=")
		if i < 0 {
			return nil, &SyntaxError{Line: n, Text: line}
		}
		key := strings.ToLower(strings.TrimSpace(line[:i]))
		val := strings.TrimSpace(line[i+1:])
		if len(val) >= 2 && val[0] == '"' {
			if j := strings.LastIndex(val, "\""); j > 0 {
				val = val[1:j]
			}
		} else {
			for k := 1; k < len(val); k++ {
				if (val[k] == ';' || val[k] == '#') && (val[k-1] == ' ' || val[k-1] == '\t') {
					val = strings.TrimSpace(val[:k])
					break
				}
			}
		}
		if f[section] == nil {
			f[section] = map[string]string{}
		}
		f[section][key] = val
	}
	return f, sc.Err()
}
