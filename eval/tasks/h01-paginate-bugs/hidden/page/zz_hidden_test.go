package page

import (
	"reflect"
	"testing"
)

func TestHiddenLastPage(t *testing.T) {
	p, _ := Get([]int{1, 2, 3, 4, 5}, 2, 3)
	if !reflect.DeepEqual(p.Items, []int{5}) || p.HasNext || !p.HasPrev || p.TotalPages != 3 {
		t.Fatalf("%+v", p)
	}
	p, _ = Get([]int{1, 2, 3}, 5, 1)
	if !reflect.DeepEqual(p.Items, []int{1, 2, 3}) {
		t.Fatalf("%+v", p)
	}
}

func TestHiddenExactMultiple(t *testing.T) {
	p, _ := Get([]int{1, 2, 3, 4}, 2, 2)
	if p.TotalPages != 2 || p.HasNext || !reflect.DeepEqual(p.Items, []int{3, 4}) {
		t.Fatalf("%+v", p)
	}
}

func TestHiddenPastEnd(t *testing.T) {
	p, err := Get([]int{1, 2, 3}, 2, 7)
	if err != nil || len(p.Items) != 0 || p.HasNext || !p.HasPrev || p.TotalPages != 2 || p.Number != 7 {
		t.Fatalf("%+v %v", p, err)
	}
	p, err = Get([]int{}, 3, 1)
	if err != nil || len(p.Items) != 0 || p.TotalPages != 0 || p.HasNext || p.HasPrev {
		t.Fatalf("empty: %+v %v", p, err)
	}
}

func TestHiddenNoAlias(t *testing.T) {
	in := []int{1, 2, 3, 4}
	p, _ := Get(in, 2, 1)
	p.Items[0] = 99
	p.Items = append(p.Items, 100)
	if in[0] != 1 || in[2] != 3 {
		t.Fatalf("aliasing: %v", in)
	}
}

func TestHiddenErrors(t *testing.T) {
	if _, err := Get([]int{1}, 0, 1); err != ErrBadSize {
		t.Fatal(err)
	}
	if _, err := Get([]int{1}, 1, 0); err != ErrBadPage {
		t.Fatal(err)
	}
}
