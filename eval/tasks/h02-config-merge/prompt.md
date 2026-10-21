Package `config` loads settings from defaults and environment variables (see `config/config.go`). Extend it:

1. Add a field `Timeout time.Duration` (env `APP_TIMEOUT`, parsed with `time.ParseDuration`, default `30s`) and a field `Tags []string` (env `APP_TAGS`, comma-separated, items trimmed, empty items dropped, default empty/nil).
2. Add `func Merge(base, override Config) Config`: every field of `override` that is non-zero replaces the field in `base` (for `Tags`: a non-nil override slice replaces base, even if empty). The function must not modify its arguments' slices.
3. `Load` must collect ALL invalid environment values and return them together as a single error whose message lists each offending variable name (e.g. using `errors.Join`), instead of stopping at the first one. When there are errors, `Load` still returns the zero `Config`.

Keep the existing behaviour for the existing fields. Add tests.
