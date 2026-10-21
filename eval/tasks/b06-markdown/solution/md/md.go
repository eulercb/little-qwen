package md

import (
	"strconv"
	"strings"
)

func ToHTML(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	return blocks(lines)
}

func isBlank(l string) bool { return strings.Trim(l, " ") == "" }

func heading(l string) (int, string, bool) {
	n := 0
	for n < len(l) && l[n] == '#' {
		n++
	}
	if n < 1 || n > 6 || (n < len(l) && l[n] != ' ') {
		return 0, "", false
	}
	t := strings.TrimRight(l[n:], " ")
	t = strings.TrimLeft(t, " ")
	j := len(t)
	for j > 0 && t[j-1] == '#' {
		j--
	}
	if j < len(t) && (j == 0 || t[j-1] == ' ') {
		t = strings.TrimRight(t[:j], " ")
	}
	return n, t, true
}

func isHR(l string) bool {
	if len(l) < 3 {
		return false
	}
	c := l[0]
	if c != '-' && c != '*' && c != '_' {
		return false
	}
	return strings.Count(l, string(c)) == len(l)
}

type itemMatch struct {
	kind  string
	start int
	text  string
}

func matchItem(l string) (itemMatch, bool) {
	if len(l) >= 2 && (l[0] == '-' || l[0] == '*' || l[0] == '+') && l[1] == ' ' {
		return itemMatch{"ul", 0, l[2:]}, true
	}
	j := 0
	for j < len(l) && l[j] >= '0' && l[j] <= '9' {
		j++
	}
	if j > 0 && j+1 < len(l) && l[j] == '.' && l[j+1] == ' ' {
		n, _ := strconv.Atoi(l[:j])
		return itemMatch{"ol", n, l[j+2:]}, true
	}
	return itemMatch{}, false
}

func indentOf(l string) int { return len(l) - len(strings.TrimLeft(l, " ")) }

type item struct {
	text   []string
	nested []*list
}

type list struct {
	kind  string
	start int
	items []*item
}

func (l *list) render(b *strings.Builder) {
	if l.kind == "ol" && l.start != 1 {
		b.WriteString(`<ol start="` + strconv.Itoa(l.start) + "\">\n")
	} else {
		b.WriteString("<" + l.kind + ">\n")
	}
	for _, it := range l.items {
		b.WriteString("<li>" + inline(it.text))
		if len(it.nested) > 0 {
			b.WriteString("\n")
			for _, n := range it.nested {
				n.render(b)
			}
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</" + l.kind + ">\n")
}

func blocks(lines []string) string {
	var b strings.Builder
	var para []string
	flush := func() {
		if len(para) > 0 {
			b.WriteString("<p>" + inline(para) + "</p>\n")
			para = nil
		}
	}
	for i := 0; i < len(lines); {
		l := lines[i]
		switch {
		case isBlank(l):
			flush()
			i++
		case strings.HasPrefix(l, "```"):
			flush()
			info := strings.TrimSpace(l[3:])
			if f := strings.Fields(info); len(f) > 0 {
				info = f[0]
			}
			i++
			var content strings.Builder
			for i < len(lines) && strings.TrimRight(lines[i], " ") != "```" {
				content.WriteString(esc(lines[i]) + "\n")
				i++
			}
			i++
			if info != "" {
				b.WriteString(`<pre><code class="language-` + esc(info) + `">` + content.String() + "</code></pre>\n")
			} else {
				b.WriteString("<pre><code>" + content.String() + "</code></pre>\n")
			}
		case func() bool { _, _, ok := heading(l); return ok }():
			flush()
			n, t, _ := heading(l)
			tag := "h" + strconv.Itoa(n)
			b.WriteString("<" + tag + ">" + inline([]string{t}) + "</" + tag + ">\n")
			i++
		case isHR(l):
			flush()
			b.WriteString("<hr />\n")
			i++
		case strings.HasPrefix(l, ">"):
			flush()
			var inner []string
			for i < len(lines) && strings.HasPrefix(lines[i], ">") {
				x := lines[i][1:]
				x = strings.TrimPrefix(x, " ")
				inner = append(inner, x)
				i++
			}
			b.WriteString("<blockquote>\n" + blocks(inner) + "</blockquote>\n")
		default:
			if m, ok := matchItem(l); ok {
				flush()
				lst := &list{kind: m.kind, start: m.start}
				for i < len(lines) {
					m, ok := matchItem(lines[i])
					if !ok || m.kind != lst.kind {
						break
					}
					it := &item{text: []string{m.text}}
					lst.items = append(lst.items, it)
					i++
					for i < len(lines) {
						cl := lines[i]
						ind := indentOf(cl)
						if isBlank(cl) || ind < 2 {
							break
						}
						rem := ind
						if rem > 4 {
							rem = 4
						}
						if nm, ok := matchItem(cl[rem:]); ok {
							var cur *list
							if n := len(it.nested); n > 0 && it.nested[n-1].kind == nm.kind {
								cur = it.nested[n-1]
							} else {
								cur = &list{kind: nm.kind, start: nm.start}
								it.nested = append(it.nested, cur)
							}
							cur.items = append(cur.items, &item{text: []string{nm.text}})
						} else if n := len(it.nested); n > 0 && ind >= 4 {
							last := it.nested[n-1].items[len(it.nested[n-1].items)-1]
							last.text = append(last.text, strings.TrimLeft(cl, " "))
						} else {
							it.text = append(it.text, strings.TrimLeft(cl, " "))
						}
						i++
					}
				}
				lst.render(&b)
				continue
			}
			para = append(para, strings.TrimLeft(l, " "))
			i++
		}
	}
	flush()
	return b.String()
}

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

const brMark = "\x00"

// inline renders the text lines of one block.
func inline(lines []string) string {
	var parts []string
	for k, l := range lines {
		last := k == len(lines)-1
		br := false
		if !last {
			if strings.HasSuffix(l, "\\") && !strings.HasSuffix(l, "\\\\") {
				l = l[:len(l)-1]
				br = true
			} else if strings.HasSuffix(l, "  ") {
				br = true
			}
		}
		l = strings.TrimRight(l, " ")
		if br {
			l += brMark
		}
		parts = append(parts, l)
	}
	out := (&inl{s: strings.Join(parts, "\n")}).run(0, len(strings.Join(parts, "\n")), true)
	return strings.ReplaceAll(out, brMark, "<br />")
}

type inl struct{ s string }

const escapable = "\\`*_{}[]()#+-.!<>"

func isWord(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
}

func runLen(s string, i int) int {
	j := i
	for j < len(s) && s[j] == s[i] {
		j++
	}
	return j - i
}

// codeSpanEnd returns the end index (exclusive) of a code span starting at i, or -1.
func codeSpanEnd(s string, i, end int) (int, int) {
	n := runLen(s, i)
	j := i + n
	for j < end {
		if s[j] == '`' {
			m := runLen(s, j)
			if m == n {
				return j, j + m
			}
			j += m
			continue
		}
		j++
	}
	return -1, -1
}

// findCloser finds a closing delimiter run of exactly k chars c in [from,end).
func (p *inl) findCloser(c byte, k, from, end int) int {
	s := p.s
	j := from
	for j < end {
		switch {
		case s[j] == '\\' && j+1 < end && strings.IndexByte(escapable, s[j+1]) >= 0:
			j += 2
			continue
		case s[j] == '`':
			if _, e := codeSpanEnd(s, j, end); e > 0 {
				j = e
				continue
			}
			j += runLen(s, j)
			continue
		case s[j] == c:
			r := runLen(s, j)
			if r > end-j {
				r = end - j
			}
			if r == k && j > from && s[j-1] != ' ' && s[j-1] != '\n' && (c != '_' || j+k >= len(s) || !isWord(s[j+k])) {
				return j
			}
			j += r
			continue
		}
		j++
	}
	return -1
}

func (p *inl) run(i, end int, links bool) string {
	s := p.s
	var b strings.Builder
	for i < end {
		c := s[i]
		switch {
		case c == '\\' && i+1 < end && strings.IndexByte(escapable, s[i+1]) >= 0:
			b.WriteString(esc(string(s[i+1])))
			i += 2
		case c == '`':
			if cs, ce := codeSpanEnd(s, i, end); cs > 0 {
				n := runLen(s, i)
				content := s[i+n : cs]
				if len(content) >= 2 && content[0] == ' ' && content[len(content)-1] == ' ' && strings.Trim(content, " ") != "" {
					content = content[1 : len(content)-1]
				}
				b.WriteString("<code>" + esc(content) + "</code>")
				i = ce
			} else {
				n := runLen(s, i)
				b.WriteString(s[i : i+n])
				i += n
			}
		case c == '<' && (strings.HasPrefix(s[i:end], "<http://") || strings.HasPrefix(s[i:end], "<https://")):
			j := i + 1
			for j < end && s[j] != '>' && s[j] != '<' && s[j] != ' ' && s[j] != '\n' {
				j++
			}
			if j < end && s[j] == '>' {
				u := s[i+1 : j]
				b.WriteString(`<a href="` + esc(u) + `">` + esc(u) + "</a>")
				i = j + 1
			} else {
				b.WriteString("&lt;")
				i++
			}
		case c == '[' && links:
			if html, next, ok := p.link(i, end); ok {
				b.WriteString(html)
				i = next
			} else {
				b.WriteString("[")
				i++
			}
		case c == '*' || c == '_':
			r := runLen(s, i)
			if i+r > end {
				r = end - i
			}
			canOpen := i+r < end && s[i+r] != ' ' && s[i+r] != '\n' && (c != '_' || i == 0 || !isWord(s[i-1]))
			done := false
			if canOpen && r >= 2 {
				if j := p.findCloser(c, 2, i+2, end); j > 0 {
					b.WriteString("<strong>" + p.run(i+2, j, links) + "</strong>")
					i = j + 2
					done = true
				}
			}
			if !done && canOpen && r == 1 {
				if j := p.findCloser(c, 1, i+1, end); j > 0 {
					b.WriteString("<em>" + p.run(i+1, j, links) + "</em>")
					i = j + 1
					done = true
				}
			}
			if !done {
				b.WriteByte(c)
				i++
			}
		default:
			b.WriteString(esc(string(c)))
			i++
		}
	}
	return b.String()
}

func (p *inl) link(i, end int) (string, int, bool) {
	s := p.s
	j := strings.IndexByte(s[i+1:end], ']')
	if j < 0 {
		return "", 0, false
	}
	te := i + 1 + j
	if te+1 >= end || s[te+1] != '(' {
		return "", 0, false
	}
	k := te + 2
	us := k
	for k < end && s[k] != ')' && s[k] != ' ' && s[k] != '\n' {
		k++
	}
	if k >= end {
		return "", 0, false
	}
	url := s[us:k]
	title := ""
	if s[k] == ' ' {
		if k+1 >= end || s[k+1] != '"' {
			return "", 0, false
		}
		q := strings.IndexByte(s[k+2:end], '"')
		if q < 0 {
			return "", 0, false
		}
		title = s[k+2 : k+2+q]
		k = k + 2 + q + 1
		if k >= end || s[k] != ')' {
			return "", 0, false
		}
		text := p.run(i+1, te, false)
		return `<a href="` + esc(url) + `" title="` + esc(title) + `">` + text + "</a>", k + 1, true
	}
	if s[k] != ')' {
		return "", 0, false
	}
	text := p.run(i+1, te, false)
	return `<a href="` + esc(url) + `">` + text + "</a>", k + 1, true
}
