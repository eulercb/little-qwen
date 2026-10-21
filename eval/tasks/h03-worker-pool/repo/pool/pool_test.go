package pool

import (
	"context"
	"testing"
)

func TestSquares(t *testing.T) {
	out, err := Map(context.Background(), []int{1, 2, 3}, 1, func(_ context.Context, x int) (int, error) { return x * x, nil })
	if err != nil || len(out) != 3 || out[2] != 9 {
		t.Fatal(out, err)
	}
}
