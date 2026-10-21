Implement a token-bucket rate limiter in package `ratelimit` (file `ratelimit/limiter.go`). The `Clock` interface already exists in `ratelimit/clock.go`; time must only be read through it so tests can use a fake clock.

API:
- `func New(rate float64, burst int, clk Clock) *Limiter` — `rate` tokens are added per second, up to `burst` tokens. The bucket starts full. If `clk` is nil use the real clock. Panic if `rate <= 0` or `burst < 1`.
- `func (l *Limiter) Allow() bool` — takes one token if available.
- `func (l *Limiter) AllowN(n int) bool` — takes n tokens atomically if all are available; `n > burst` always returns false; `n <= 0` returns true and takes nothing.
- `func (l *Limiter) Tokens() float64` — currently available tokens (after refilling up to now).
- `func (l *Limiter) Reserve() time.Duration` — takes one token now even if that makes the balance negative, and returns how long the caller must wait until that token would have been available (0 if one was available).

Tokens refill continuously (fractional tokens accumulate). The limiter must be safe for concurrent use. Write tests using a fake clock.
