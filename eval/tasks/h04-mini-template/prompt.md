Create package `tmpl` (directory `tmpl/`) — a tiny text template engine. Standard library only (do not use `text/template`).

```go
func Render(src string, data map[string]any) (string, error)
```

- Text outside `{{ ... }}` is copied verbatim. `{{{{` outputs a literal `{{` (and is not the start of a tag).
- A tag is `{{ name | filter | filter:"arg" }}`; whitespace inside the braces is ignored around names, `|` and `:`.
- `name` is `[A-Za-z_][A-Za-z0-9_]*` optionally followed by `.field` segments (`user.name`) that index nested `map[string]any` values.
- Value formatting: `string` as is; `int`, `int64`, `float64` via `fmt.Sprint`; `bool` as `true`/`false`; `nil` or a missing name → empty string (no error), unless a `default` filter supplies a value.
- Filters, applied left to right on the string value:
  - `upper`, `lower`, `trim` (strings.TrimSpace)
  - `default:"x"` — if the current value is the empty string, use `x`
  - `truncate:"N"` — keep at most N runes, appending `…` (U+2026) if anything was cut (the `…` is not counted in N)
  - `html` — escape `& < > " '` as `&amp; &lt; &gt; &#34; &#39;`
- Argument strings are double-quoted and support `\"` and `\\` escapes.
- Errors (returned as a non-nil error, with empty output): an unknown filter (message contains the filter name), a missing or extra argument for a filter, a non-integer or negative `truncate` argument, an unterminated tag (`{{` without `}}`), an empty tag `{{ }}`, a bad name, or indexing `.field` into a value that is not a map (message contains the full name, e.g. `user.name.first`).
Add tests.
