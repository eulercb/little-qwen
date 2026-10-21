# expr — specification

## API (package `expr`)
```go
type Error struct {
    Pos int    // 0-based byte offset in the source where the problem is
    Msg string
}
func (e *Error) Error() string // format: "<Pos>: <Msg>"

type Program struct { /* unexported */ }
func Compile(src string) (*Program, error)            // parse once; returns *Error on syntax errors
func (p *Program) Eval(vars map[string]float64) (float64, error) // evaluate many times; *Error on runtime errors
func (p *Program) Vars() []string                      // sorted, de-duplicated variable names referenced
func Eval(src string, vars map[string]float64) (float64, error) // Compile + Eval
```
All errors returned by the API must be of type `*Error`.

## Lexical structure
- Whitespace (space, tab, newline) separates tokens and is otherwise ignored.
- Numbers: `123`, `1.5`, `.5`, `1.`, `1e3`, `2.5E-2`. (No sign in the literal; `-` is an operator.)
- Identifiers: `[A-Za-z_][A-Za-z0-9_]*`. `true` and `false` are literals equal to 1 and 0.
- Operators/punctuation: `+ - * / % ^ ( ) , ? : < <= > >= == != && || !`
- Any other character is a syntax error at its position: message `unexpected character 'X'`.

## Grammar and precedence (lowest to highest)
1. Ternary `c ? a : b` — right-associative (`a ? b : c ? d : e` = `a ? b : (c ? d : e)`).
2. `||`
3. `&&`
4. Equality `==`, `!=`
5. Comparison `<`, `<=`, `>`, `>=`
6. Additive `+`, `-` (left-assoc)
7. Multiplicative `*`, `/`, `%` (left-assoc)
8. Unary prefix `-`, `+`, `!`
9. Power `^` — right-associative and binds tighter than unary on its left: `-2^2 = -4`, `2^-1 = 0.5` (the right operand of `^` may itself be a unary expression), `2^3^2 = 512`.
10. Primary: number, identifier, function call `name(arg, ...)`, parenthesised expression.

## Semantics
- All values are float64. Booleans are 1 (true) and 0 (false); any non-zero value is truthy.
- `%` is `math.Mod`. `^` is `math.Pow`.
- Comparison/equality/logical operators return 1 or 0. `!x` is 1 if x == 0 else 0.
- `&&`, `||` and `?:` short-circuit: the operand that is not needed is NOT evaluated (so it cannot produce runtime errors).
- Division by zero (`/` or `%` with a zero right operand) is a runtime error at the position of the operator: message `division by zero`.
- Unknown variable: runtime error at the identifier's position: message `unknown variable "name"`.
- Results that are NaN or ±Inf are a runtime error at position 0: message `result is not finite`.

## Built-in functions
| name | arity | meaning |
|---|---|---|
| `abs(x)` | 1 | absolute value |
| `sqrt(x)` | 1 | square root; negative argument → runtime error at the function name: `sqrt of negative number` |
| `min(a, ...)` / `max(a, ...)` | ≥ 1 | minimum / maximum |
| `round(x)` | 1 | `math.Round` |
| `clamp(x, lo, hi)` | 3 | `min(max(x, lo), hi)` |

- Unknown function: **compile** error at the name: `unknown function "name"`.
- Wrong number of arguments: **compile** error at the name: `function "name" expects N arguments, got M` (for min/max: `expects at least 1 argument, got 0`).
- Function names are not variables: `abs` alone (no call) is a variable reference.

## Syntax errors (Compile)
- Unexpected token: `unexpected token "X"` at the token's position (for end of input use `unexpected end of input` at position len(src)).
- Missing `)`: `expected ")"` at the position where it was expected (the position of the token found instead, or len(src)).
- Missing `:` in a ternary: `expected ":"` likewise.
- Empty input (only whitespace): `unexpected end of input` at len(src).
- Trailing tokens after a complete expression: `unexpected token "X"`.
