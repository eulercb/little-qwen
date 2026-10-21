package words

import (
	"bufio"
	"io"
	"sort"
	"strings"
	"unicode"
)

type WordCount struct {
	Word  string
	Count int
}

func TopK(r io.Reader, k int, stop []string) ([]WordCount, error) {
	if k <= 0 {
		return []WordCount{}, nil
	}
	sw := map[string]bool{}
	for _, s := range stop {
		sw[strings.ToLower(s)] = true
	}
	counts := map[string]int{}
	br := bufio.NewReader(r)
	var cur []rune
	pendingApos := false
	flush := func() {
		if len(cur) > 0 {
			w := strings.ToLower(string(cur))
			if !sw[w] {
				counts[w]++
			}
		}
		cur = cur[:0]
		pendingApos = false
	}
	for {
		c, _, err := br.ReadRune()
		if err != nil {
			flush()
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch {
		case unicode.IsLetter(c) || unicode.IsDigit(c):
			if pendingApos {
				if unicode.IsLetter(c) && len(cur) > 0 && unicode.IsLetter(cur[len(cur)-1]) {
					cur = append(cur, '\'')
				} else {
					flush()
				}
				pendingApos = false
			}
			cur = append(cur, c)
		case c == '\'' || c == '’':
			if pendingApos || len(cur) == 0 {
				flush()
			} else {
				pendingApos = true
			}
		default:
			flush()
		}
	}
	out := make([]WordCount, 0, len(counts))
	for w, n := range counts {
		out = append(out, WordCount{w, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Word < out[j].Word
	})
	if len(out) > k {
		out = out[:k]
	}
	return out, nil
}
