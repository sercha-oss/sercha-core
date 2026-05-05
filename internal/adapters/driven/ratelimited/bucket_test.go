package ratelimited

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestBucket_Wait_ImmediateWhenBudgetPlentiful verifies that Wait returns
// without blocking when the bucket has sufficient capacity.
func TestBucket_Wait_ImmediateWhenBudgetPlentiful(t *testing.T) {
	b := NewBucket(1000, 100)

	start := time.Now()
	if err := b.Wait(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Wait took too long: %v (expected <50ms)", elapsed)
	}
}

// TestBucket_Wait_WeightedConsumption verifies that a weighted Wait acquires
// the right amount from the bucket.
func TestBucket_Wait_WeightedConsumption(t *testing.T) {
	// Large capacity, slow refill — weight consumption must be immediate for
	// a capacity that covers the weight.
	b := NewBucket(500, 1) // 500 initial capacity, 1 token/sec refill

	start := time.Now()
	if err := b.Wait(context.Background(), 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("Wait(weight=100) took too long: %v (expected <100ms)", elapsed)
	}
}

// TestBucket_Wait_CtxCancelledMidBlock verifies that Wait returns ctx.Err()
// when the context is cancelled while the bucket is exhausted.
func TestBucket_Wait_CtxCancelledMidBlock(t *testing.T) {
	// Very low capacity and very slow refill to ensure Wait blocks.
	b := NewBucket(1, 0.001) // 1 token, refills at 0.001/sec (1000s per token)

	// Drain the single token.
	ctx := context.Background()
	if err := b.Wait(ctx, 1); err != nil {
		t.Fatalf("unexpected error draining bucket: %v", err)
	}

	// Now cancel the context and try to acquire more — should return quickly.
	cancelCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := b.Wait(cancelCtx, 1)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error when context cancelled, got nil")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Wait did not honour context cancellation: elapsed %v", elapsed)
	}
}

// TestBucket_Update_ZeroRemainingCausesNextWaitToBlock verifies that calling
// Update(0, now+10s) causes the next Wait to wait for the reset window.
func TestBucket_Update_ZeroRemainingCausesNextWaitToBlock(t *testing.T) {
	b := NewBucket(1000, 100)

	// Set remaining=0, reset in 200ms.
	resetAt := time.Now().Add(200 * time.Millisecond)
	b.Update(0, resetAt)

	start := time.Now()
	// Use a context with a generous timeout so we don't interfere.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := b.Wait(ctx, 1)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have waited at least ~200ms (the override window).
	if elapsed < 150*time.Millisecond {
		t.Errorf("Wait returned too quickly (%v) after Update(remaining=0): expected ~200ms", elapsed)
	}
}

// TestBucket_Update_Reconciliation verifies that Update with a positive
// remaining value does not block subsequent Waits for small amounts.
func TestBucket_Update_Reconciliation(t *testing.T) {
	b := NewBucket(100, 10)

	// Report a healthy remaining budget with a far-future reset.
	resetAt := time.Now().Add(60 * time.Second)
	b.Update(500, resetAt)

	start := time.Now()
	if err := b.Wait(context.Background(), 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("Wait blocked after positive Update: elapsed %v", elapsed)
	}
}

// TestBucket_ConcurrentWaits verifies that multiple goroutines can Wait
// concurrently without data races.
func TestBucket_ConcurrentWaits(t *testing.T) {
	b := NewBucket(10000, 1000)

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.Wait(ctx, 1); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Wait returned error: %v", err)
	}
}

// TestBucket_ConcurrentUpdateAndWait verifies that concurrent Updates and
// Waits do not cause data races (race detector test).
func TestBucket_ConcurrentUpdateAndWait(t *testing.T) {
	b := NewBucket(5000, 500)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// Concurrent Waits
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Wait(ctx, 1)
		}()
	}

	// Concurrent Updates
	for i := range 5 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			remaining := int64(100 * (i + 1))
			b.Update(remaining, time.Now().Add(time.Second))
		}(i)
	}

	wg.Wait()
}

// TestBucket_Wait_ZeroWeightUsesOne verifies that a weight of 0 or negative
// is treated as weight 1 and does not cause errors.
func TestBucket_Wait_ZeroWeightUsesOne(t *testing.T) {
	b := NewBucket(100, 10)

	if err := b.Wait(context.Background(), 0); err != nil {
		t.Fatalf("unexpected error for zero weight: %v", err)
	}
	if err := b.Wait(context.Background(), -5); err != nil {
		t.Fatalf("unexpected error for negative weight: %v", err)
	}
}
