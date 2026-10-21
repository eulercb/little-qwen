# Environment facts
- Go toolchain is installed; module downloads are disabled (GOPROXY=off), so use only the standard library and modules already in go.mod.
- `go test ./...` runs all tests; add `-run TestName` to focus and `-count=1` to bypass the test cache.
