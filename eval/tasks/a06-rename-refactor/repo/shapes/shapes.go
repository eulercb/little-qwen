// Package shapes provides basic 2D shapes.
package shapes

import (
	"errors"
	"math"
)

// Shp is a 2D shape.
type Shp interface {
	Ar() float64
	Per() float64
}

// Circle is a circle with radius R.
type Circle struct{ R float64 }

func (c Circle) Ar() float64  { return math.Pi * c.R * c.R }
func (c Circle) Per() float64 { return 2 * math.Pi * c.R }

// Rect is an axis-aligned rectangle.
type Rect struct{ W, H float64 }

func (r Rect) Ar() float64  { return r.W * r.H }
func (r Rect) Per() float64 { return 2 * (r.W + r.H) }

// Triangle has side lengths A, B, C.
type Triangle struct{ A, B, C float64 }

// NewTriangle validates the triangle inequality.
func NewTriangle(a, b, c float64) (Triangle, error) {
	if a <= 0 || b <= 0 || c <= 0 || a+b <= c || a+c <= b || b+c <= a {
		return Triangle{}, errors.New("shapes: invalid triangle")
	}
	return Triangle{a, b, c}, nil
}

func (t Triangle) Per() float64 { return t.A + t.B + t.C }

// Ar uses Heron's formula.
func (t Triangle) Ar() float64 {
	s := t.Per() / 2
	return math.Sqrt(s * (s - t.A) * (s - t.B) * (s - t.C))
}

// TotalAr sums the areas.
func TotalAr(shapes []Shp) float64 {
	sum := 0.0
	for _, s := range shapes {
		sum += s.Ar()
	}
	return sum
}
