package ratelimited

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Bucket is a concrete driven.Limiter backed by golang.org/x/time/rate.
//
// It maintains two layers of state:
//
//  1. A *rate.Limiter that models steady-state refill at a constant rate.
//     All Wait calls are serialised through this limiter so backpressure is
//     applied uniformly across goroutines.
//
//  2. A mutex-guarded override window: when Update is called with
//     authoritative server-side remaining/resetAt values, those values
//     temporarily cap the bucket until resetAt. During the override window
//     Wait returns ErrRateLimitOverride if remaining is zero, and drains
//     the override counter before consulting the rate.Limiter.
//
// Precision note: int(weight) truncates weights larger than math.MaxInt32 on
// 32-bit platforms. Token counts are at most ~10^6 per request in practice,
// well within the safe range on every supported platform.
//
// Bucket is safe for concurrent use.
type Bucket struct {
	lim         *rate.Limiter
	refillPerSec float64
	capacity     int64

	mu        sync.Mutex
	override  bool
	remaining int64
	resetAt   time.Time
}

// NewBucket creates a new token-bucket Limiter with the given initial capacity
// and refill rate.
//
// initialCapacity is the number of tokens available at construction time.
// The underlying rate.Limiter is created with burst=initialCapacity so all
// tokens are immediately available.
//
// refillPerSec is the steady-state token refill rate per second; for a
// tokens-per-minute budget, pass float64(tpm)/60.
func NewBucket(initialCapacity int64, refillPerSec float64) *Bucket {
	cap := int(initialCapacity)
	if cap < 1 {
		cap = 1
	}
	// rate.NewLimiter starts with burst tokens fully available.
	lim := rate.NewLimiter(rate.Limit(refillPerSec), cap)
	return &Bucket{
		lim:          lim,
		refillPerSec: refillPerSec,
		capacity:     initialCapacity,
	}
}

// Wait blocks until the budget has enough capacity to service a request of
// the given weight, or until ctx is done.
//
// When an Update override is active and the override remaining count would
// be exhausted by this weight, Wait first blocks until resetAt, then
// proceeds through the underlying rate.Limiter. This prevents the caller
// from blasting past a server-confirmed budget of zero.
//
// ctx.Done() is always honoured — Wait never sleeps through cancellation.
func (b *Bucket) Wait(ctx context.Context, weight int64) error {
	if weight <= 0 {
		weight = 1
	}

	var sleepUntil time.Time
	var sleepNeeded bool
	effectiveWeight := weight

	b.mu.Lock()
	if b.override {
		now := time.Now()
		if now.Before(b.resetAt) {
			if b.remaining <= 0 {
				// Override says exhausted — must sleep until reset.
				sleepUntil = b.resetAt
				sleepNeeded = true
				// Clear the override so the next Wait after reset goes
				// straight to the rate.Limiter.
				b.override = false
			} else {
				// Consume from override remaining.
				consume := weight
				if consume > b.remaining {
					consume = b.remaining
				}
				b.remaining -= consume
				if b.remaining < 0 {
					b.remaining = 0
				}
				effectiveWeight = consume
			}
		} else {
			// Override window has expired.
			b.override = false
		}
	}
	b.mu.Unlock()

	if sleepNeeded {
		slog.Debug("ratelimited: override exhausted, sleeping until reset",
			"reset_at", sleepUntil)
		timer := time.NewTimer(time.Until(sleepUntil))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			// Proceed to rate.Limiter.WaitN after the reset window.
		}
	}

	slog.Debug("ratelimited: acquiring tokens from rate limiter", "weight", effectiveWeight)
	return b.lim.WaitN(ctx, int(effectiveWeight))
}

// Update reconciles the bucket's internal state with the authoritative
// server-reported remaining budget and reset time.
//
// Update is fire-and-forget: it does not block, does not return an error, and
// serialises correctly with concurrent Wait calls via the internal mutex.
//
// After Update, the next Wait will respect the server-reported remaining count
// until resetAt. At resetAt the override window expires and the underlying
// rate.Limiter resumes normal operation.
func (b *Bucket) Update(remaining int64, resetAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if resetAt.Before(now) {
		// Server-reported reset is in the past — skip the override to avoid
		// blocking on a stale window.
		b.override = false
		return
	}

	b.override = true
	b.remaining = remaining
	b.resetAt = resetAt

	slog.Debug("ratelimited: bucket updated from server response",
		"remaining", remaining,
		"reset_at", resetAt)

	// Reconcile the rate.Limiter's internal state: if the server says we
	// have fewer tokens than the limiter thinks, drain the excess so the
	// next WaitN(ctx, weight) reflects reality.
	current := b.lim.TokensAt(now)
	if float64(remaining) < current {
		drain := current - float64(remaining)
		b.lim.AllowN(now, int(drain))
	}
}
