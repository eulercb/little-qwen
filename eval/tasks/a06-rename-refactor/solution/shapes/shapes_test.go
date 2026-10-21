package shapes

import (
	"math"
	"testing"
)

func TestAreas(t *testing.T) {
	tri, err := NewTriangle(3, 4, 5)
	if err != nil {
		t.Fatal(err)
	}
	got := TotalArea([]Shape{Rectangle{3, 4}, tri})
	if math.Abs(got-18) > 1e-9 {
		t.Fatalf("total = %v", got)
	}
	if math.Abs(Circle{1}.Perimeter()-2*math.Pi) > 1e-9 {
		t.Fatal("circle perimeter")
	}
}
