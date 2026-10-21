package tmpl

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var nameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

func Render(src string, data map[string]any) (string, error) {
	var b strings.Builder
	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], "{{{{") {
			b.WriteString("{{")
			i += 4
			continue
		}
		if strings.HasPrefix(src[i:], "{{") {
			j := strings.Index(src[i+2:], "}}")
			if j < 0 {
				return "", errors.New("tmpl: unterminated tag")
			}
			out, err := evalTag(src[i+2:i+2+j], data)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
			i += 2 + j + 2
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String(), nil
}

func splitPipes(s string) []string {
	var parts []string
	var cur strings.Builder
	inQ := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQ {
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
				continue
			}
			if c == '"' {
				inQ = false
			}
			continue
		}
		if c == '"' {
			inQ = true
		}
		if c == '|' {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	return append(parts, cur.String())
}

func unquote(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("tmpl: bad argument %s", s)
	}
	var b strings.Builder
	for i := 1; i < len(s)-1; i++ {
		if s[i] == '\\' && i+1 < len(s)-1 {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}

func lookup(name string, data map[string]any) (any, error) {
	segs := strings.Split(name, ".")
	var cur any = data
	for k, s := range segs {
		m, ok := cur.(map[string]any)
		if !ok {
			if cur == nil && k > 0 {
				return nil, nil
			}
			return nil, fmt.Errorf("tmpl: cannot index %s", name)
		}
		cur = m[s]
		if cur == nil && k < len(segs)-1 {
			return nil, nil
		}
	}
	return cur, nil
}

func format(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprint(x)
	}
}

func evalTag(tag string, data map[string]any) (string, error) {
	parts := splitPipes(tag)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", errors.New("tmpl: empty tag")
	}
	if !nameRe.MatchString(name) {
		return "", fmt.Errorf("tmpl: bad name %q", name)
	}
	v, err := lookup(name, data)
	if err != nil {
		return "", err
	}
	s := format(v)
	for _, f := range parts[1:] {
		fname, arg, hasArg := strings.Cut(f, ":")
		fname = strings.TrimSpace(fname)
		needArg := fname == "default" || fname == "truncate"
		if needArg != hasArg {
			return "", fmt.Errorf("tmpl: filter %s: wrong number of arguments", fname)
		}
		var a string
		if hasArg {
			if a, err = unquote(arg); err != nil {
				return "", err
			}
		}
		switch fname {
		case "upper":
			s = strings.ToUpper(s)
		case "lower":
			s = strings.ToLower(s)
		case "trim":
			s = strings.TrimSpace(s)
		case "html":
			s = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;").Replace(s)
		case "default":
			if s == "" {
				s = a
			}
		case "truncate":
			n, err := strconv.Atoi(a)
			if err != nil || n < 0 {
				return "", fmt.Errorf("tmpl: truncate: bad length %q", a)
			}
			r := []rune(s)
			if len(r) > n {
				s = string(r[:n]) + "…"
			}
		default:
			return "", fmt.Errorf("tmpl: unknown filter %q", fname)
		}
	}
	return s, nil
}
