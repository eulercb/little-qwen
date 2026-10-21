// Command area prints the total area of some shapes.
package main

import (
	"fmt"

	"example.com/shapes/shapes"
)

func main() {
	all := []shapes.Shape{shapes.Circle{R: 1}, shapes.Rectangle{W: 2, H: 3}}
	fmt.Printf("total area: %.2f\n", shapes.TotalArea(all))
}
