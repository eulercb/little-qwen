`pool.Map` (in `pool/pool.go`) runs a function over inputs with a fixed number of workers. It has three reported problems:

1. Results sometimes come back in the wrong order — the contract says `out[i]` must be the result for `in[i]`.
2. When the context is cancelled, `Map` can hang forever, and worker goroutines leak.
3. When a call fails, `Map` should stop starting new work and return the error from the lowest input index that failed (with an `*IndexError` wrapping it), but currently it returns whichever error happens to arrive first.

Fix `Map` so it honours the documented contract in its doc comment. Keep the signature. Add tests (use `-race`).
