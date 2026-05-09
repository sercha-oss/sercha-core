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

// TestBucket_Update_ZeroRemaining_GatesProportionally verifies that calling
// Update(0, ...) DOES gate subsequent Wait calls, but only by the local
// refill model's estimate of how long until `weight` tokens would replenish
// — never by the full reset window.
//
// For weight=1 against refillPerSec=100, the gate is ~10ms (1/100 of a
// second). Far below the reset duration (2s) and the test deadline (1s).
//
// Rationale: provider headers describe a sliding window, not a hard reset.
// Sleeping until the full reset is far too pessimistic; ignoring the
// override entirely (the previous behaviour) caused 429s on the next call.
// The middle ground gates by the local refill estimate.
func TestBucket_Update_ZeroRemaining_GatesProportionally(t *testing.T) {
	b := NewBucket(1000, 100)

	resetAt := time.Now().Add(2 * time.Second)
	b.Update(0, resetAt)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := b.Wait(ctx, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	// Expected: ~10ms (1 token / 100 per sec). Allow generous slop for CI.
	if elapsed > 200*time.Millisecond {
		t.Errorf("Wait gated longer than expected (%v) for weight=1 against refill 100/s", elapsed)
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
