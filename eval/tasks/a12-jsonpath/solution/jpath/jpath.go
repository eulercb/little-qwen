package jpath

import (
	"errors"
	"sort"
	"strconv"
)

type step struct {
	kind  int // 0 member,1 index,2 wildcard,3 recursive member
	name  string
	index int
}

var errSyntax = errors.New("jpath: syntax error")

func isIdentStart(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func isIdent(c byte) bool      { return isIdentStart(c) || c >= '0' && c <= '9' }

func parse(p string) ([]step, error) {
	if len(p) == 0 || p[0] != '$' {
		return nil, errSyntax
	}
	var steps []step
	i := 1
	ident := func() (string, bool) {
		if i >= len(p) || !isIdentStart(p[i]) {
			return "", false
		}
		j := i
		for j < len(p) && isIdent(p[j]) {
			j++
		}
		s := p[i:j]
		i = j
		return s, true
	}
	for i < len(p) {
		switch p[i] {
		case '.':
			i++
			if i < len(p) && p[i] == '.' {
				i++
				n, ok := ident()
				if !ok {
					return nil, errSyntax
				}
				steps = append(steps, step{kind: 3, name: n})
				continue
			}
			if i < len(p) && p[i] == '*' {
				i++
				steps = append(steps, step{kind: 2})
				continue
			}
			n, ok := ident()
			if !ok {
				return nil, errSyntax
			}
			steps = append(steps, step{kind: 0, name: n})
		case '[':
			i++
			if i >= len(p) {
				return nil, errSyntax
			}
			switch {
			case p[i] == '*':
				i++
				steps = append(steps, step{kind: 2})
			case p[i] == '"':
				i++
				var b []byte
				closed := false
				for i < len(p) {
					c := p[i]
					if c == '\\' && i+1 < len(p) && (p[i+1] == '"' || p[i+1] == '\\') {
						b = append(b, p[i+1])
						i += 2
						continue
					}
					if c == '"' {
						closed = true
						i++
						break
					}
					b = append(b, c)
					i++
				}
				if !closed {
					return nil, errSyntax
				}
				steps = append(steps, step{kind: 0, name: string(b)})
			default:
				j := i
				if j < len(p) && p[j] == '-' {
					j++
				}
				for j < len(p) && p[j] >= '0' && p[j] <= '9' {
					j++
				}
				n, err := strconv.Atoi(p[i:j])
				if err != nil {
					return nil, errSyntax
				}
				i = j
				steps = append(steps, step{kind: 1, index: n})
			}
			if i >= len(p) || p[i] != ']' {
				return nil, errSyntax
			}
			i++
		default:
			return nil, errSyntax
		}
	}
	return steps, nil
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func descend(n any, name string, out *[]any) {
	switch v := n.(type) {
	case map[string]any:
		if x, ok := v[name]; ok {
			*out = append(*out, x)
		}
		for _, k := range sortedKeys(v) {
			descend(v[k], name, out)
		}
	case []any:
		for _, x := range v {
			descend(x, name, out)
		}
	}
}

func Query(doc any, path string) ([]any, error) {
	steps, err := parse(path)
	if err != nil {
		return nil, err
	}
	cur := []any{doc}
	for _, s := range steps {
		next := []any{}
		for _, n := range cur {
			switch s.kind {
			case 0:
				if m, ok := n.(map[string]any); ok {
					if x, ok := m[s.name]; ok {
						next = append(next, x)
					}
				}
			case 1:
				if a, ok := n.([]any); ok {
					idx := s.index
					if idx < 0 {
						idx += len(a)
					}
					if idx >= 0 && idx < len(a) {
						next = append(next, a[idx])
					}
				}
			case 2:
				switch v := n.(type) {
				case []any:
					next = append(next, v...)
				case map[string]any:
					for _, k := range sortedKeys(v) {
						next = append(next, v[k])
					}
				}
			case 3:
				descend(n, s.name, &next)
			}
		}
		cur = next
	}
	return cur, nil
}
