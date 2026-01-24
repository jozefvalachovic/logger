package audit

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu              sync.Mutex
	tokens          float64
	maxTokens       float64
	refillRate      float64
	lastRefill      time.Time
	dropWhenLimited bool
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg *RateLimitConfig) *RateLimiter {
	if cfg == nil {
		return nil
	}

	burstSize := cfg.BurstSize
	if burstSize <= 0 {
		burstSize = cfg.EventsPerSecond * 2
	}

	return &RateLimiter{
		tokens:          float64(burstSize),
		maxTokens:       float64(burstSize),
		refillRate:      float64(cfg.EventsPerSecond),
		lastRefill:      time.Now(),
		dropWhenLimited: cfg.DropWhenLimited,
	}
}

// Allow checks if an event is allowed and consumes a token
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

// AllowN checks if n events are allowed and consumes n tokens
func (r *RateLimiter) AllowN(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	needed := float64(n)
	if r.tokens >= needed {
		r.tokens -= needed
		return true
	}

	return false
}

// Wait blocks until a token is available or context is canceled
func (r *RateLimiter) Wait() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return 0
	}

	needed := 1.0 - r.tokens
	waitTime := time.Duration(needed/r.refillRate*1000) * time.Millisecond

	return waitTime
}

// DropWhenLimited returns whether events should be dropped when rate limited
func (r *RateLimiter) DropWhenLimited() bool {
	return r.dropWhenLimited
}

// TokensAvailable returns the current number of available tokens
func (r *RateLimiter) TokensAvailable() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	return r.tokens
}

func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.lastRefill = now

	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
}
