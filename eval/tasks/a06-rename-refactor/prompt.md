Refactor package `shapes` without changing behaviour:

1. Rename the interface `Shp` to `Shape`, and its method `Ar()` to `Area()` and `Per()` to `Perimeter()`, everywhere (all implementations, callers, tests, and `cmd/area`).
2. Rename the type `Rect` to `Rectangle` (fields stay `W` and `H`).
3. Add a function `func Largest(shapes []Shape) (Shape, bool)` returning the shape with the greatest area (the first one on ties) and false for an empty slice.
4. Add a `String() string` method to `Circle`, `Rectangle` and `Triangle` returning e.g. `Circle(r=1.5)`, `Rectangle(3x4)`, `Triangle(3,4,5)` — numbers formatted with `strconv.FormatFloat(x, 'f', -1, 64)`.

The module must build and all tests must pass.
