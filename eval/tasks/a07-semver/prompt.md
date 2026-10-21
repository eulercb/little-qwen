Create package `semver` (file `semver/semver.go`, module root already has `go.mod`) implementing Semantic Versioning 2.0.0 parsing and precedence.

API:
- `type Version struct { Major, Minor, Patch uint64; Pre []string; Build []string }`
- `func Parse(s string) (Version, error)` — accepts an optional leading `v`. `MAJOR.MINOR.PATCH`, optional `-pre.release.ids`, optional `+build.ids`. Reject: leading zeros in numeric parts (`01.2.3`) and in numeric pre-release identifiers (`1.0.0-01`), empty identifiers (`1.0.0-alpha..1`), characters outside `[0-9A-Za-z-]` in identifiers, missing parts (`1.2`).
- `func (v Version) String() string` — canonical form without the `v` prefix.
- `func Compare(a, b Version) int` — -1, 0, +1 by SemVer precedence: compare major, minor, patch numerically; a version with a pre-release has lower precedence than the same version without; pre-release identifiers are compared left to right: numeric identifiers numerically, alphanumeric lexically in ASCII order, numeric < alphanumeric, and a shorter list is lower if all preceding identifiers are equal. Build metadata is ignored.
- `func MustParse(s string) Version` — panics on error.

Include your own tests in `semver/semver_test.go`.
