// Package ratelimit provides a token-bucket rate limiter.
package ratelimit

import "time"

// Clock abstracts time for testing.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RealClock is the wall clock.
var RealClock Clock = realClock{}
