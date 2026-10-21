package page

import "testing"

func TestFirstPage(t *testing.T) {
	p, err := Get([]int{1, 2, 3, 4, 5}, 2, 1)
	if err != nil || len(p.Items) != 2 || !p.HasNext || p.HasPrev {
		t.Fatalf("%+v %v", p, err)
	}
}
