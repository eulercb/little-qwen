// Package shapes provides basic 2D shapes.
package shapes

import (
	"errors"
	"math"
	"strconv"
)

// Shape is a 2D shape.
type Shape interface {
	Area() float64
	Perimeter() float64
}

// Circle is a circle with radius R.
type Circle struct{ R float64 }

func (c Circle) Area() float64  { return math.Pi * c.R * c.R }
func (c Circle) Perimeter() float64 { return 2 * math.Pi * c.R }

// Rectangle is an axis-aligned rectangle.
type Rectangle struct{ W, H float64 }

func (r Rectangle) Area() float64  { return r.W * r.H }
func (r Rectangle) Perimeter() float64 { return 2 * (r.W + r.H) }

// Triangle has side lengths A, B, C.
type Triangle struct{ A, B, C float64 }

// NewTriangle validates the triangle inequality.
func NewTriangle(a, b, c float64) (Triangle, error) {
	if a <= 0 || b <= 0 || c <= 0 || a+b <= c || a+c <= b || b+c <= a {
		return Triangle{}, errors.New("shapes: invalid triangle")
	}
	return Triangle{a, b, c}, nil
}

func (t Triangle) Perimeter() float64 { return t.A + t.B + t.C }

// Ar uses Heron's formula.
func (t Triangle) Area() float64 {
	s := t.Perimeter() / 2
	return math.Sqrt(s * (s - t.A) * (s - t.B) * (s - t.C))
}

// TotalArea sums the areas.
func TotalArea(shapes []Shape) float64 {
	sum := 0.0
	for _, s := range shapes {
		sum += s.Area()
	}
	return sum
}

func Largest(shapes []Shape) (Shape, bool) {
	if len(shapes) == 0 {
		return nil, false
	}
	best := shapes[0]
	for _, s := range shapes[1:] {
		if s.Area() > best.Area() {
			best = s
		}
	}
	return best, true
}

func ff(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }

func (c Circle) String() string    { return "Circle(r=" + ff(c.R) + ")" }
func (r Rectangle) String() string { return "Rectangle(" + ff(r.W) + "x" + ff(r.H) + ")" }
func (t Triangle) String() string  { return "Triangle(" + ff(t.A) + "," + ff(t.B) + "," + ff(t.C) + ")" }
