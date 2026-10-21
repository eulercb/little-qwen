package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu     sync.Mutex
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
	clk    Clock
}

func New(rate float64, burst int, clk Clock) *Limiter {
	if rate <= 0 || burst < 1 {
		panic("ratelimit: bad args")
	}
	if clk == nil {
		clk = RealClock
	}
	return &Limiter{rate: rate, burst: float64(burst), tokens: float64(burst), last: clk.Now(), clk: clk}
}

func (l *Limiter) refill() {
	now := l.clk.Now()
	if el := now.Sub(l.last).Seconds(); el > 0 {
		l.tokens += el * l.rate
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
	}
	l.last = now
}

func (l *Limiter) Allow() bool { return l.AllowN(1) }

func (l *Limiter) AllowN(n int) bool {
	if n <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if float64(n) > l.burst {
		return false
	}
	l.refill()
	if l.tokens >= float64(n) {
		l.tokens -= float64(n)
		return true
	}
	return false
}

func (l *Limiter) Tokens() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	return l.tokens
}

func (l *Limiter) Reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.refill()
	l.tokens--
	if l.tokens >= 0 {
		return 0
	}
	return time.Duration(-l.tokens / l.rate * float64(time.Second))
}
