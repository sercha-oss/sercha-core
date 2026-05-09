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
// It composes three layers of back-pressure, all of which Wait honours:
//
//  1. A *rate.Limiter sized at refillPerSec tokens/second (TPM/60). This is
//     the local model of steady-state token throughput. Always honoured.
//
//  2. An optional second *rate.Limiter sized at requestsPerMinute/60
//     (the request-rate ceiling). When non-nil, every Wait call also
//     consumes one request slot — useful because providers commonly
//     publish independent TPM and RPM limits and either can be the
//     binding constraint.
//
//  3. A mutex-guarded override window: when Update is called with the
//     server-reported remaining/resetAt values (typically from
//     x-ratelimit-* headers), Wait holds for an estimated proportional
//     delay when remaining < weight. The estimate uses refillPerSec as the
//     refill model and is capped by the resetAt so we never sleep longer
//     than the server's own counter cares about.
//
// Precision note: int(weight) and int(requestsPerMinute) truncate values
// larger than math.MaxInt32 on 32-bit platforms. Realistic configurations
// (TPM up to ~10M, RPM up to ~10K, per-request weight up to ~10^6) sit
// comfortably inside the safe range on every supported platform.
//
// Bucket is safe for concurrent use.
type Bucket struct {
	lim          *rate.Limiter // tokens per second (TPM/60)
	reqLim       *rate.Limiter // requests per second (RPM/60); nil disables RPM gating
	refillPerSec float64
	capacity     int64

	mu        sync.Mutex
	override  bool
	remaining int64
	resetAt   time.Time
}

// TPM returns the configured token capacity (initial burst). Useful for
// option helpers that want to preserve TPM settings while replacing the
// bucket — e.g. when adding an RPM gate to an existing bucket.
func (b *Bucket) TPM() int64 { return b.capacity }

// RefillPerSec returns the configured token refill rate per second. Same
// preservation use case as TPM().
func (b *Bucket) RefillPerSec() float64 { return b.refillPerSec }

// NewBucket creates a token-bucket limiter with the given initial token
// capacity and token refill rate. RPM gating is disabled.
//
// initialCapacity is the number of tokens available at construction time.
// The underlying rate.Limiter is created with burst=initialCapacity so all
// tokens are immediately available.
//
// refillPerSec is the steady-state token refill rate per second; for a
// tokens-per-minute budget, pass float64(tpm)/60.
func NewBucket(initialCapacity int64, refillPerSec float64) *Bucket {
	return NewBucketWithRPM(initialCapacity, refillPerSec, 0)
}

// NewBucketWithRPM creates a token-bucket limiter that gates on BOTH tokens
// per minute and requests per minute. Each Wait call consumes `weight` tokens
// from the TPM bucket and 1 request from the RPM bucket.
//
// requestsPerMinute <= 0 disables RPM gating (equivalent to NewBucket).
//
// Burst on the request limiter is set to `requestsPerMinute` so a fresh
// bucket can absorb a brief burst up to the per-minute ceiling — mirrors how
// the TPM bucket works.
func NewBucketWithRPM(initialCapacity int64, refillPerSec float64, requestsPerMinute int64) *Bucket {
	capTokens := int(initialCapacity)
	if capTokens < 1 {
		capTokens = 1
	}
	lim := rate.NewLimiter(rate.Limit(refillPerSec), capTokens)

	var reqLim *rate.Limiter
	if requestsPerMinute > 0 {
		burst := int(requestsPerMinute)
		if burst < 1 {
			burst = 1
		}
		reqPerSec := float64(requestsPerMinute) / 60.0
		reqLim = rate.NewLimiter(rate.Limit(reqPerSec), burst)
	}

	return &Bucket{
		lim:          lim,
		reqLim:       reqLim,
		refillPerSec: refillPerSec,
		capacity:     initialCapacity,
	}
}

// Wait blocks until the budget has enough capacity to service a request of
// the given weight, or until ctx is done.
//
// Two layers of back-pressure compose:
//
//  1. The underlying rate.Limiter self-paces at refillPerSec. Always honoured.
//
//  2. The override window (populated by Update from server-reported headers)
//     applies an additional cap when the server says we're below `weight`
//     remaining. We wait long enough for our local refill model to plausibly
//     have credited enough tokens to bring (remaining + refilled) >= weight,
//     bounded by the server-reported reset time. This avoids two failure
//     modes: (a) ignoring the override entirely and getting 429s on the next
//     call, and (b) sleeping for the full reset window on a single
//     near-exhaustion signal — which is far too pessimistic because providers
//     like OpenAI publish a sliding window, not a hard reset.
//
//     The override counter is decremented by the consumed weight so successive
//     Wait calls under one override window track usage correctly.
//
// 429 responses (hard back-pressure on actual exhaustion) are still handled
// by the Transport via Retry-After.
//
// ctx.Done() is always honoured.
func (b *Bucket) Wait(ctx context.Context, weight int64) error {
	if weight <= 0 {
		weight = 1
	}

	// Compute any required override-driven delay under the lock.
	b.mu.Lock()
	var overrideDelay time.Duration
	now := time.Now()
	if b.override {
		if now.Before(b.resetAt) {
			if b.remaining < weight {
				// Estimate seconds until the local refill model would have
				// credited (weight - remaining) tokens. Cap at the server's
				// reset to avoid waiting longer than the override window.
				needed := weight - b.remaining
				if b.refillPerSec > 0 {
					estSec := float64(needed) / b.refillPerSec
					overrideDelay = time.Duration(estSec * float64(time.Second))
				}
				if untilReset := b.resetAt.Sub(now); overrideDelay > untilReset {
					overrideDelay = untilReset
				}
				if overrideDelay < 0 {
					overrideDelay = 0
				}
			}
			// Decrement the override counter regardless — so consecutive Waits
			// under one override window track real usage, not just gate timing.
			b.remaining -= weight
			if b.remaining < 0 {
				b.remaining = 0
			}
		} else {
			b.override = false
		}
	}
	b.mu.Unlock()

	if overrideDelay > 0 {
		slog.Debug("ratelimited: holding for server-override window",
			"weight", weight,
			"delay_ms", overrideDelay.Milliseconds(),
		)
		t := time.NewTimer(overrideDelay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}

	// RPM gate first — cheapest to fail (1 token, fast). If the request rate
	// is the binding constraint, we'd rather block here than after waiting
	// on the heavier TPM bucket.
	if b.reqLim != nil {
		slog.Debug("ratelimited: acquiring 1 request slot from RPM limiter")
		if err := b.reqLim.WaitN(ctx, 1); err != nil {
			return err
		}
	}

	slog.Debug("ratelimited: acquiring tokens from TPM limiter", "weight", weight)
	return b.lim.WaitN(ctx, int(weight))
}

// Update reconciles the bucket's internal state with the server-reported
// remaining budget and reset time.
//
// Update is fire-and-forget: it does not block, does not return an error,
// and serialises correctly with concurrent Wait calls via the internal mutex.
//
// Behaviour:
//   - The override counter is set so the bucket tracks per-window consumption
//     against the server-reported remaining count. Subsequent Wait calls
//     hold for a proportional delay (relative to refillPerSec, capped by
//     resetAt) when the recorded remaining is below the requested weight.
//     This makes the override an actual gate, not a passive accounting
//     counter — the previous design ignored it and produced 429s on the
//     next call.
//   - The local rate.Limiter is drained when the server reports fewer
//     tokens than it believes are available, so the next WaitN reflects
//     the tighter actual cap.
//   - The local limiter is never refilled from a positive server signal —
//     refillPerSec stays the steady-state floor and a transiently
//     generous server response can't cause us to exceed it.
//   - The RPM limiter (if any) is not touched by Update — providers do
//     not publish a per-request equivalent of x-ratelimit-remaining, so
//     the local refill model is the only signal.
func (b *Bucket) Update(remaining int64, resetAt time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if resetAt.Before(now) {
		// Server-reported reset is in the past — clear any active override.
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
	// next WaitN(ctx, weight) reflects reality. AllowN consumes the tokens
	// it returns true on, then returns immediately without blocking.
	current := b.lim.TokensAt(now)
	if float64(remaining) < current {
		drain := current - float64(remaining)
		b.lim.AllowN(now, int(drain))
	}
}
