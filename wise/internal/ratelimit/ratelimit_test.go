package ratelimit_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iamkanishka/wise-client-go/wise/internal/ratelimit"
)

func TestTokenBucket_AllowsRequests(t *testing.T) {
	tb := ratelimit.New(100, 10) // 100 req/s, burst 10

	ctx := context.Background()

	for range 10 {
		if err := tb.Wait(ctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestTokenBucket_BlocksWhenEmpty(t *testing.T) {
	tb := ratelimit.New(1, 1) // 1 req/s, burst 1

	ctx := context.Background()

	// First request consumes the only token.
	if err := tb.Wait(ctx); err != nil {
		t.Fatalf("first Wait: %v", err)
	}

	// Second request should block briefly then succeed.
	start := time.Now()
	if err := tb.Wait(ctx); err != nil {
		t.Fatalf("second Wait: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 500*time.Millisecond {
		t.Errorf("expected blocking delay, got %v", elapsed)
	}
}

func TestTokenBucket_ContextCancellation(t *testing.T) {
	tb := ratelimit.New(0.001, 1) // very slow refill — forces wait

	ctx := context.Background()

	// Drain the bucket.
	if err := tb.Wait(ctx); err != nil {
		t.Fatalf("drain: %v", err)
	}

	// Now try with a very short timeout.
	shortCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()

	if err := tb.Wait(shortCtx); err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestDefaultBucket_NonNil(t *testing.T) {
	tb := ratelimit.DefaultBucket()
	if tb == nil {
		t.Fatal("DefaultBucket returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := tb.Wait(ctx); err != nil {
		t.Fatalf("DefaultBucket.Wait: %v", err)
	}
}

func TestTokenBucket_ConcurrentSafe(t *testing.T) {
	tb := ratelimit.New(100, 50)
	ctx := context.Background()

	var completed int32

	done := make(chan struct{})

	for range 20 {
		go func() {
			if err := tb.Wait(ctx); err == nil {
				atomic.AddInt32(&completed, 1)
			}
			done <- struct{}{}
		}()
	}

	for range 20 {
		<-done
	}

	if atomic.LoadInt32(&completed) == 0 {
		t.Error("expected at least some requests to complete")
	}
}
