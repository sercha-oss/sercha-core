package driven

import (
	"context"
	"time"
)

// Limiter is a generic, provider-agnostic rate-limit budget. It serves two
// roles to the consumer:
//
//  1. Pre-flight back-pressure: callers ask Wait(ctx, weight) before making
//     a rate-limited call. Wait blocks until the budget can service the
//     weighted request, or returns ctx.Err() if the context is cancelled
//     while waiting. Concurrent Waits must serialise through the underlying
//     bucket so multiple goroutines share one budget.
//
//  2. Post-response reconciliation: after a successful response from the
//     remote service, the consumer calls Update with the authoritative
//     remaining-budget and reset-time signalled by the service (typically
//     read from x-ratelimit-* response headers). The Limiter adjusts its
//     internal accounting so subsequent Waits reflect the server's view.
//
// The "weight" is provider-defined and opaque to the Limiter. For
// token-priced APIs it is the estimated token count of the request; for
// request-priced APIs it is 1; for cost-budget Limiters it would be the
// estimated dollar cost. The Limiter does not interpret the units.
//
// Implementations must be safe for concurrent use. Update is fire-and-
// forget — implementations may not return errors, must not block on I/O,
// and must serialise correctly with in-flight Waits.
type Limiter interface {
	Wait(ctx context.Context, weight int64) error
	Update(remaining int64, resetAt time.Time)
}
