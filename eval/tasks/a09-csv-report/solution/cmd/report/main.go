package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func cents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, errors.New("bad price")
	}
	c := int64(0)
	if len(parts) == 2 {
		f := parts[1]
		if len(f) == 0 || len(f) > 2 {
			return 0, errors.New("bad price")
		}
		if len(f) == 1 {
			f += "0"
		}
		c, err = strconv.ParseInt(f, 10, 64)
		if err != nil {
			return 0, errors.New("bad price")
		}
	}
	return whole*100 + c, nil
}

func money(c int64) string { return fmt.Sprintf("%d.%02d", c/100, c%100) }

func run(args []string, in io.Reader, out io.Writer) error {
	by, top := "region", 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--by":
			if i+1 >= len(args) {
				return errors.New("--by needs a value")
			}
			i++
			by = args[i]
			if by != "region" && by != "product" {
				return fmt.Errorf("bad --by %q", by)
			}
		case "--top":
			if i+1 >= len(args) {
				return errors.New("--top needs a value")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("bad --top %q", args[i])
			}
			top = n
		default:
			return fmt.Errorf("unknown argument %q", args[i])
		}
	}
	r := csv.NewReader(in)
	r.FieldsPerRecord = -1
	hdr, err := r.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}
	col := map[string]int{}
	for i, h := range hdr {
		col[strings.TrimSpace(h)] = i
	}
	for _, need := range []string{"region", "product", "qty", "price"} {
		if _, ok := col[need]; !ok {
			return fmt.Errorf("missing column %q", need)
		}
	}
	type agg struct {
		qty, rev int64
	}
	groups := map[string]*agg{}
	var tq, tr int64
	line := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		q, err := strconv.ParseInt(strings.TrimSpace(rec[col["qty"]]), 10, 64)
		if err != nil || q < 0 {
			return fmt.Errorf("line %d: bad qty", line)
		}
		p, err := cents(rec[col["price"]])
		if err != nil {
			return fmt.Errorf("line %d: bad price", line)
		}
		k := rec[col[by]]
		g := groups[k]
		if g == nil {
			g = &agg{}
			groups[k] = g
		}
		g.qty += q
		g.rev += q * p
		tq += q
		tr += q * p
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := groups[keys[i]], groups[keys[j]]
		if a.rev != b.rev {
			return a.rev > b.rev
		}
		return keys[i] < keys[j]
	})
	if top > 0 && top < len(keys) {
		keys = keys[:top]
	}
	for _, k := range keys {
		fmt.Fprintf(out, "%s\t%d\t%s\n", k, groups[k].qty, money(groups[k].rev))
	}
	fmt.Fprintf(out, "TOTAL\t%d\t%s\n", tq, money(tr))
	return nil
}
