---
name: go-pitfalls
description: Go semantics that silently break spec details (nil vs empty slices, append, maps, JSON, string parsing). Read before finishing any Go task whose spec mentions nil/empty values, copying, escaping or quoting.
---
# Go pitfalls that pass weak tests but violate specs

## nil vs empty
- `append([]T(nil), src...)` returns **nil** when `src` is empty. To copy while keeping "non-nil but empty",
  use `out := make([]T, len(src)); copy(out, src)` (non-nil even for len 0) — or check `src != nil` first.
- `var s []T` is nil; `s := []T{}` is non-nil. `json.Marshal` gives `null` vs `[]`. `reflect.DeepEqual(nil, []T{})` is false.
- `%v`/`%+v` print nil and empty slices identically (`[]`). Test nil-ness with `s == nil`, never by printing.
- A spec saying "X is nil when …" or "non-nil even if empty" needs an explicit test asserting `== nil` / `!= nil`.

## Copy and alias
- `out := in` shares the backing array; `in[a:b]` aliases. If callers may modify the result, copy.
- Maps are references: copying a struct that holds a map shares the map.

## Parsing with quotes/escapes
- Tokenise **before** splitting on separators. Splitting `a | f:"x | y"` on `|` first breaks quoted arguments.
  Scan character by character, tracking "inside quotes" and backslash escapes, and split only outside quotes.
- Handle `\"` and `\\` inside quotes; decide what an unknown escape means from the spec.

## Errors
- Wrap with `fmt.Errorf("...: %w", err)` so `errors.Is/As` work; `errors.Join` for "report all errors".

## Checklist before finish
For every sentence in the spec that states a rule, edge case or error, point to a test that checks exactly it.
Add the missing tests, run them, then finish.
