package main

import (
	"bytes"
	"strings"
	"testing"
)

const zzhData = `product,qty,region,price,note
apple,3,north,1.10,x
pear,1,south,2.00,
apple,10,south,0.10,
kiwi,2,east,1.5,
plum,1,west,1.00,
`

func zzhRunS(t *testing.T, args []string, in string) (string, error) {
	t.Helper()
	var b bytes.Buffer
	err := run(args, strings.NewReader(in), &b)
	return b.String(), err
}

func TestHiddenByRegion(t *testing.T) {
	got, err := zzhRunS(t, nil, zzhData)
	if err != nil {
		t.Fatal(err)
	}
	want := "north\t3\t3.30\neast\t2\t3.00\nsouth\t11\t3.00\nwest\t1\t1.00\nTOTAL\t17\t10.30\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestHiddenByProductTop(t *testing.T) {
	got, err := zzhRunS(t, []string{"--by", "product", "--top", "2"}, zzhData)
	if err != nil {
		t.Fatal(err)
	}
	want := "apple\t13\t4.30\nkiwi\t2\t3.00\nTOTAL\t17\t10.30\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

func TestHiddenCentsPrecision(t *testing.T) {
	in := "region,product,qty,price\na,x,3,0.10\na,y,7,0.10\n"
	got, _ := zzhRunS(t, nil, in)
	if got != "a\t10\t1.00\nTOTAL\t10\t1.00\n" {
		t.Fatalf("%q", got)
	}
}

func TestHiddenErrors(t *testing.T) {
	if _, err := zzhRunS(t, nil, "region,product,qty\na,b,1\n"); err == nil || !strings.Contains(err.Error(), "price") {
		t.Errorf("missing column: %v", err)
	}
	if _, err := zzhRunS(t, nil, "region,product,qty,price\na,b,1,1.00\na,b,x,1.00\n"); err == nil || !strings.Contains(err.Error(), "3") {
		t.Errorf("bad qty line: %v", err)
	}
	if _, err := zzhRunS(t, nil, "region,product,qty,price\na,b,1,abc\n"); err == nil || !strings.Contains(err.Error(), "2") {
		t.Errorf("bad price line: %v", err)
	}
	if _, err := zzhRunS(t, []string{"--by", "color"}, zzhData); err == nil {
		t.Error("bad --by accepted")
	}
	if _, err := zzhRunS(t, []string{"--top", "0"}, zzhData); err == nil {
		t.Error("bad --top accepted")
	}
}
