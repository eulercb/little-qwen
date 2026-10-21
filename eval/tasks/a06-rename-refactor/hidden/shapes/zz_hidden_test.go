package shapes

import (
	"math"
	"testing"
)

func TestHiddenInterface(t *testing.T) {
	var _ Shape = Circle{}
	var _ Shape = Rectangle{}
	var _ Shape = Triangle{}
	r := Rectangle{W: 2, H: 5}
	if r.Area() != 10 || r.Perimeter() != 14 {
		t.Fatal("rectangle")
	}
}

func TestHiddenLargest(t *testing.T) {
	if _, ok := Largest(nil); ok {
		t.Fatal("empty")
	}
	a := Rectangle{W: 2, H: 2}
	b := Rectangle{W: 1, H: 4}
	c := Circle{R: 0.5}
	got, ok := Largest([]Shape{c, a, b})
	if !ok || got != Shape(a) {
		t.Fatalf("largest = %v", got)
	}
}

func TestHiddenStrings(t *testing.T) {
	tri, _ := NewTriangle(3, 4, 5)
	cases := map[string]interface{ String() string }{
		"Circle(r=1.5)":     Circle{R: 1.5},
		"Rectangle(3x4)":    Rectangle{W: 3, H: 4},
		"Triangle(3,4,5)":   tri,
		"Rectangle(0.25x2)": Rectangle{W: 0.25, H: 2},
	}
	for want, s := range cases {
		if got := s.String(); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}

func TestHiddenBehaviourKept(t *testing.T) {
	if _, err := NewTriangle(1, 2, 3); err == nil {
		t.Fatal("degenerate triangle accepted")
	}
	tri, _ := NewTriangle(3, 4, 5)
	if math.Abs(tri.Area()-6) > 1e-9 {
		t.Fatal("heron")
	}
}
