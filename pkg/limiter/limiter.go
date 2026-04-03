package limiter

import (
	"context"
	"sync"
	"time"
)

type RateLimiter struct {
	rate       float64
	tokens     float64
	maxTokens  float64
	lastUpdate time.Time
	mu         sync.Mutex
}

func NewRateLimiter(requestsPerSecond int) *RateLimiter {
	return &RateLimiter{
		rate:       float64(requestsPerSecond),
		tokens:     float64(requestsPerSecond),
		maxTokens:  float64(requestsPerSecond),
		lastUpdate: time.Now(),
	}
}

func (r *RateLimiter) Wait(ctx context.Context) error {
	return r.WaitN(ctx, 1)
}

func (r *RateLimiter) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	need := float64(n)

	r.mu.Lock()

	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.tokens += elapsed * r.rate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastUpdate = now

	if r.tokens >= need {
		r.tokens -= need
		r.mu.Unlock()
		return nil
	}

	missing := need - r.tokens
	waitTime := time.Duration(missing / r.rate * float64(time.Second))

	r.mu.Unlock()

	select {
	case <-time.After(waitTime):
		return r.WaitN(ctx, n)
	case <-ctx.Done():
		return ctx.Err()
	}
}
