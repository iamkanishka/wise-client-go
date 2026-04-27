// Package ratelimit provides a thread-safe token-bucket rate limiter
// for controlling the rate of outbound HTTP requests.
package ratelimit

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Limiter is implemented by any value that can gate outbound requests.
// The built-in [TokenBucket] satisfies this interface.
type Limiter interface {
	// Wait blocks until one token is available or ctx is canceled.
	Wait(ctx context.Context) error
}

// TokenBucket implements a thread-safe token-bucket rate limiter.
//
// Tokens are replenished at a constant rate up to the configured burst
// capacity. Each call to Wait consumes one token. When the bucket is
// empty, Wait blocks until a token becomes available.
type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// New creates a TokenBucket that permits ratePerSec requests per second
// with an initial and maximum burst capacity of burst.
//
// Example — 10 req/s sustained with burst of 20:
//
//	bucket := ratelimit.New(10, 20)
func New(ratePerSec, burst float64) *TokenBucket {
	return &TokenBucket{
		mu:         sync.Mutex{},
		tokens:     burst,
		maxTokens:  burst,
		refillRate: ratePerSec,
		lastRefill: time.Now(),
	}
}

// DefaultBucket returns a TokenBucket tuned for the Wise API:
// 10 requests/second sustained, burst of 20.
func DefaultBucket() *TokenBucket {
	return New(10, 20)
}

// Wait blocks until a token is available or ctx is canceled.
// It returns ctx.Err() if the context expires before a token is available.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("ratelimit: context canceled: %w", err)
		}

		tb.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(tb.lastRefill).Seconds()
		tb.tokens = math.Min(tb.maxTokens, tb.tokens+elapsed*tb.refillRate)
		tb.lastRefill = now

		if tb.tokens >= 1 {
			tb.tokens--
			tb.mu.Unlock()

			return nil
		}

		// Calculate how long until we have a token.
		waitFor := time.Duration((1-tb.tokens)/tb.refillRate*1000) * time.Millisecond
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return fmt.Errorf("ratelimit: context canceled: %w", ctx.Err())
		case <-time.After(waitFor):
		}
	}
}
