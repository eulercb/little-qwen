package expr

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

type Error struct {
	Pos int
	Msg string
}

func (e *Error) Error() string { return fmt.Sprintf("%d: %s", e.Pos, e.Msg) }

type tok struct {
	kind string // num ident op eof
	text string
	num  float64
	pos  int
}

func lex(src string) ([]tok, error) {
	var ts []tok
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c >= '0' && c <= '9' || c == '.' && i+1 < len(src) && src[i+1] >= '0' && src[i+1] <= '9':
			j := i
			for j < len(src) && src[j] >= '0' && src[j] <= '9' {
				j++
			}
			if j < len(src) && src[j] == '.' {
				j++
				for j < len(src) && src[j] >= '0' && src[j] <= '9' {
					j++
				}
			}
			if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
				k := j + 1
				if k < len(src) && (src[k] == '+' || src[k] == '-') {
					k++
				}
				if k < len(src) && src[k] >= '0' && src[k] <= '9' {
					for k < len(src) && src[k] >= '0' && src[k] <= '9' {
						k++
					}
					j = k
				}
			}
			v, err := strconv.ParseFloat(src[i:j], 64)
			if err != nil {
				return nil, &Error{i, "bad number"}
			}
			ts = append(ts, tok{"num", src[i:j], v, i})
			i = j
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i
			for j < len(src) && (src[j] == '_' || src[j] >= 'a' && src[j] <= 'z' || src[j] >= 'A' && src[j] <= 'Z' || src[j] >= '0' && src[j] <= '9') {
				j++
			}
			ts = append(ts, tok{"ident", src[i:j], 0, i})
			i = j
		default:
			two := ""
			if i+1 < len(src) {
				two = src[i : i+2]
			}
			switch two {
			case "<=", ">=", "==", "!=", "&&", "||":
				ts = append(ts, tok{"op", two, 0, i})
				i += 2
				continue
			}
			switch c {
			case '+', '-', '*', '/', '%', '^', '(', ')', ',', '?', ':', '<', '>', '!':
				ts = append(ts, tok{"op", string(c), 0, i})
				i++
			default:
				return nil, &Error{i, fmt.Sprintf("unexpected character '%c'", c)}
			}
		}
	}
	ts = append(ts, tok{"eof", "", 0, len(src)})
	return ts, nil
}

type node interface{}
type numN struct{ v float64 }
type varN struct {
	name string
	pos  int
}
type unN struct {
	op string
	x  node
}
type binN struct {
	op   string
	pos  int
	l, r node
}
type ternN struct{ c, a, b node }
type callN struct {
	name string
	pos  int
	args []node
}

type parser struct {
	ts   []tok
	i    int
	vars map[string]bool
}

func (p *parser) peek() tok { return p.ts[p.i] }
func (p *parser) next() tok { t := p.ts[p.i]; p.i++; return t }
func (p *parser) isOp(s string) bool {
	t := p.peek()
	return t.kind == "op" && t.text == s
}

func unexpected(t tok) error {
	if t.kind == "eof" {
		return &Error{t.pos, "unexpected end of input"}
	}
	return &Error{t.pos, fmt.Sprintf("unexpected token %q", t.text)}
}

func (p *parser) expect(s string) error {
	if !p.isOp(s) {
		return &Error{p.peek().pos, fmt.Sprintf("expected %q", s)}
	}
	p.i++
	return nil
}

func (p *parser) ternary() (node, error) {
	c, err := p.or()
	if err != nil {
		return nil, err
	}
	if p.isOp("?") {
		p.i++
		a, err := p.ternary()
		if err != nil {
			return nil, err
		}
		if err := p.expect(":"); err != nil {
			return nil, err
		}
		b, err := p.ternary()
		if err != nil {
			return nil, err
		}
		return ternN{c, a, b}, nil
	}
	return c, nil
}

func (p *parser) binLevel(ops []string, sub func() (node, error)) (node, error) {
	l, err := sub()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		match := false
		for _, o := range ops {
			if t.kind == "op" && t.text == o {
				match = true
			}
		}
		if !match {
			return l, nil
		}
		p.i++
		r, err := sub()
		if err != nil {
			return nil, err
		}
		l = binN{t.text, t.pos, l, r}
	}
}

func (p *parser) or() (node, error)  { return p.binLevel([]string{"||"}, p.and) }
func (p *parser) and() (node, error) { return p.binLevel([]string{"&&"}, p.eq) }
func (p *parser) eq() (node, error)  { return p.binLevel([]string{"==", "!="}, p.cmp) }
func (p *parser) cmp() (node, error) { return p.binLevel([]string{"<", "<=", ">", ">="}, p.add) }
func (p *parser) add() (node, error) { return p.binLevel([]string{"+", "-"}, p.mul) }
func (p *parser) mul() (node, error) { return p.binLevel([]string{"*", "/", "%"}, p.unary) }

func (p *parser) unary() (node, error) {
	if p.isOp("-") || p.isOp("+") || p.isOp("!") {
		t := p.next()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return unN{t.text, x}, nil
	}
	return p.power()
}

func (p *parser) power() (node, error) {
	b, err := p.primary()
	if err != nil {
		return nil, err
	}
	if p.isOp("^") {
		t := p.next()
		e, err := p.unary()
		if err != nil {
			return nil, err
		}
		return binN{"^", t.pos, b, e}, nil
	}
	return b, nil
}

var arity = map[string][2]int{"abs": {1, 1}, "sqrt": {1, 1}, "min": {1, -1}, "max": {1, -1}, "round": {1, 1}, "clamp": {3, 3}}

func (p *parser) primary() (node, error) {
	t := p.next()
	switch t.kind {
	case "num":
		return numN{t.num}, nil
	case "ident":
		if p.isOp("(") {
			p.i++
			var args []node
			if !p.isOp(")") {
				for {
					a, err := p.ternary()
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					if p.isOp(",") {
						p.i++
						continue
					}
					break
				}
			}
			if err := p.expect(")"); err != nil {
				return nil, err
			}
			ar, ok := arity[t.text]
			if !ok {
				return nil, &Error{t.pos, fmt.Sprintf("unknown function %q", t.text)}
			}
			if ar[1] < 0 && len(args) < ar[0] {
				return nil, &Error{t.pos, fmt.Sprintf("function %q expects at least 1 argument, got %d", t.text, len(args))}
			}
			if ar[1] >= 0 && len(args) != ar[0] {
				return nil, &Error{t.pos, fmt.Sprintf("function %q expects %d arguments, got %d", t.text, ar[0], len(args))}
			}
			return callN{t.text, t.pos, args}, nil
		}
		switch t.text {
		case "true":
			return numN{1}, nil
		case "false":
			return numN{0}, nil
		}
		p.vars[t.text] = true
		return varN{t.text, t.pos}, nil
	case "op":
		if t.text == "(" {
			x, err := p.ternary()
			if err != nil {
				return nil, err
			}
			if err := p.expect(")"); err != nil {
				return nil, err
			}
			return x, nil
		}
	}
	return nil, unexpected(t)
}

type Program struct {
	root node
	vars []string
}

func Compile(src string) (*Program, error) {
	ts, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{ts: ts, vars: map[string]bool{}}
	n, err := p.ternary()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != "eof" {
		return nil, unexpected(p.peek())
	}
	var vs []string
	for v := range p.vars {
		vs = append(vs, v)
	}
	sort.Strings(vs)
	return &Program{n, vs}, nil
}

func (p *Program) Vars() []string { return append([]string(nil), p.vars...) }

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func eval(n node, vars map[string]float64) (float64, error) {
	switch x := n.(type) {
	case numN:
		return x.v, nil
	case varN:
		v, ok := vars[x.name]
		if !ok {
			return 0, &Error{x.pos, fmt.Sprintf("unknown variable %q", x.name)}
		}
		return v, nil
	case unN:
		v, err := eval(x.x, vars)
		if err != nil {
			return 0, err
		}
		switch x.op {
		case "-":
			return -v, nil
		case "!":
			return b2f(v == 0), nil
		}
		return v, nil
	case ternN:
		c, err := eval(x.c, vars)
		if err != nil {
			return 0, err
		}
		if c != 0 {
			return eval(x.a, vars)
		}
		return eval(x.b, vars)
	case callN:
		var args []float64
		for _, a := range x.args {
			v, err := eval(a, vars)
			if err != nil {
				return 0, err
			}
			args = append(args, v)
		}
		switch x.name {
		case "abs":
			return math.Abs(args[0]), nil
		case "sqrt":
			if args[0] < 0 {
				return 0, &Error{x.pos, "sqrt of negative number"}
			}
			return math.Sqrt(args[0]), nil
		case "round":
			return math.Round(args[0]), nil
		case "clamp":
			return math.Min(math.Max(args[0], args[1]), args[2]), nil
		case "min", "max":
			r := args[0]
			for _, a := range args[1:] {
				if x.name == "min" {
					r = math.Min(r, a)
				} else {
					r = math.Max(r, a)
				}
			}
			return r, nil
		}
	case binN:
		l, err := eval(x.l, vars)
		if err != nil {
			return 0, err
		}
		switch x.op {
		case "&&":
			if l == 0 {
				return 0, nil
			}
			r, err := eval(x.r, vars)
			return b2f(r != 0), err
		case "||":
			if l != 0 {
				return 1, nil
			}
			r, err := eval(x.r, vars)
			return b2f(r != 0), err
		}
		r, err := eval(x.r, vars)
		if err != nil {
			return 0, err
		}
		switch x.op {
		case "+":
			return l + r, nil
		case "-":
			return l - r, nil
		case "*":
			return l * r, nil
		case "/":
			if r == 0 {
				return 0, &Error{x.pos, "division by zero"}
			}
			return l / r, nil
		case "%":
			if r == 0 {
				return 0, &Error{x.pos, "division by zero"}
			}
			return math.Mod(l, r), nil
		case "^":
			return math.Pow(l, r), nil
		case "<":
			return b2f(l < r), nil
		case "<=":
			return b2f(l <= r), nil
		case ">":
			return b2f(l > r), nil
		case ">=":
			return b2f(l >= r), nil
		case "==":
			return b2f(l == r), nil
		case "!=":
			return b2f(l != r), nil
		}
	}
	return 0, &Error{0, "internal error"}
}

func (p *Program) Eval(vars map[string]float64) (float64, error) {
	v, err := eval(p.root, vars)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, &Error{0, "result is not finite"}
	}
	return v, nil
}

func Eval(src string, vars map[string]float64) (float64, error) {
	p, err := Compile(src)
	if err != nil {
		return 0, err
	}
	return p.Eval(vars)
}
