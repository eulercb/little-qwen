Implement `func Parse(args []string) (Config, error)` in `cli/cli.go` (the `Config` type and `ErrUsage` already exist there). Do not use the `flag` package; write the parser by hand.

Rules:
- `-v` or `--verbose` sets `Verbose = true`. Repeating it is allowed.
- `-n N`, `-n=N`, `--count N`, `--count=N` set `Count` (integer ≥ 0). Default `Count` is 1. A missing value, a non-integer, or a negative value is an error.
- `-o PATH`, `-o=PATH`, `--output PATH`, `--output=PATH` set `Output`. Default `"-"`.
- `--tag T` (or `--tag=T`) may be repeated; values append to `Tags` in order. `Tags` is nil when no tag is given.
- Short boolean flags can be combined: `-vv` is the same as `-v -v`. (Only `v` is boolean.)
- `--` ends option parsing; everything after it is positional, even if it starts with `-`.
- A lone `-` is a positional argument.
- Any other argument starting with `-` is an unknown flag → error.
- Positional arguments go to `Args` in order (`Args` is nil when there are none).

All errors must wrap `ErrUsage` (so `errors.Is(err, ErrUsage)` is true) and mention the offending flag in the message.
