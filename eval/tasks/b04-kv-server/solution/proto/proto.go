package proto

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"example.com/kv/store"
)

var errSyntax = errors.New("syntax error")

func tokenize(line string) ([]string, error) {
	var toks []string
	i := 0
	for i < len(line) {
		c := line[i]
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		if c == '"' {
			i++
			var b strings.Builder
			closed := false
			for i < len(line) {
				c := line[i]
				if c == '\\' && i+1 < len(line) {
					switch line[i+1] {
					case '"':
						b.WriteByte('"')
					case '\\':
						b.WriteByte('\\')
					case 'n':
						b.WriteByte('\n')
					default:
						b.WriteByte('\\')
						b.WriteByte(line[i+1])
					}
					i += 2
					continue
				}
				if c == '"' {
					closed = true
					i++
					break
				}
				b.WriteByte(c)
				i++
			}
			if !closed {
				return nil, errSyntax
			}
			toks = append(toks, b.String())
			continue
		}
		j := i
		for j < len(line) && line[j] != ' ' && line[j] != '\t' {
			j++
		}
		toks = append(toks, line[i:j])
		i = j
	}
	return toks, nil
}

func quote(v string) string {
	if v != "" && !strings.ContainsAny(v, " \t\"\\\n") {
		return v
	}
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n")
	return "\"" + r.Replace(v) + "\""
}

func str(v string) string     { return "$" + quote(v) + "\n" }
func integer(n int64) string  { return ":" + strconv.FormatInt(n, 10) + "\n" }
func errResp(m string) string { return "-ERR " + m + "\n" }

func Exec(s *store.Store, line string) string {
	toks, err := tokenize(line)
	if err != nil {
		return errResp("syntax error")
	}
	if len(toks) == 0 {
		return errResp("empty command")
	}
	name := strings.ToLower(toks[0])
	args := toks[1:]
	wrong := errResp("wrong number of arguments for '" + name + "'")
	notInt := errResp("value is not an integer")
	switch name {
	case "ping":
		if len(args) != 0 {
			return wrong
		}
		return "+PONG\n"
	case "set":
		if len(args) != 2 && len(args) != 4 {
			if len(args) < 2 {
				return wrong
			}
			return errResp("syntax error")
		}
		var ttl time.Duration
		if len(args) == 4 {
			if strings.ToLower(args[2]) != "ex" {
				return errResp("syntax error")
			}
			n, err := strconv.ParseInt(args[3], 10, 64)
			if err != nil {
				return notInt
			}
			if n <= 0 {
				return errResp("syntax error")
			}
			ttl = time.Duration(n) * time.Second
		}
		s.Set(args[0], args[1], ttl)
		return "+OK\n"
	case "get":
		if len(args) != 1 {
			return wrong
		}
		v, ok := s.Get(args[0])
		if !ok {
			return "$nil\n"
		}
		return str(v)
	case "del":
		if len(args) < 1 {
			return wrong
		}
		return integer(int64(s.Del(args...)))
	case "exists":
		if len(args) != 1 {
			return wrong
		}
		if s.Exists(args[0]) {
			return integer(1)
		}
		return integer(0)
	case "expire":
		if len(args) != 2 {
			return wrong
		}
		n, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return notInt
		}
		if s.Expire(args[0], time.Duration(n)*time.Second) {
			return integer(1)
		}
		return integer(0)
	case "persist":
		if len(args) != 1 {
			return wrong
		}
		if s.Persist(args[0]) {
			return integer(1)
		}
		return integer(0)
	case "ttl":
		if len(args) != 1 {
			return wrong
		}
		d := s.TTL(args[0])
		if d < 0 {
			return integer(int64(d))
		}
		secs := int64(d / time.Second)
		if d%time.Second != 0 {
			secs++
		}
		return integer(secs)
	case "incr", "incrby":
		by := int64(1)
		if name == "incr" && len(args) != 1 || name == "incrby" && len(args) != 2 {
			return wrong
		}
		if name == "incrby" {
			n, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				return notInt
			}
			by = n
		}
		n, err := s.Incr(args[0], by)
		if err != nil {
			return notInt
		}
		return integer(n)
	case "keys":
		if len(args) != 1 {
			return wrong
		}
		ks := s.Keys(args[0])
		var b strings.Builder
		b.WriteString("*" + strconv.Itoa(len(ks)) + "\n")
		for _, k := range ks {
			b.WriteString(str(k))
		}
		return b.String()
	case "dbsize":
		if len(args) != 0 {
			return wrong
		}
		return integer(int64(s.Len()))
	}
	return errResp("unknown command '" + toks[0] + "'")
}
