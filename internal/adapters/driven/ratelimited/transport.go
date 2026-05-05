package ratelimited

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// HeaderTokenWeight is the request header the embedder (or any provider client)
// sets to communicate the estimated token cost of a request to the Transport's
// Weight callback. The header is read by the Weight function and is not
// forwarded to the remote server — callers must strip it from the canonical
// request if the remote may reject unknown headers, though in practice most
// providers silently ignore unrecognised headers.
//
// The value is a base-10 integer string representing the estimated token count.
const HeaderTokenWeight = "X-Sercha-Token-Weight"

// ErrBodyNotReplayable is returned when the transport needs to retry a request
// that has a body but no GetBody function. Callers must ensure req.GetBody is
// set for any request that may be retried (http.NewRequestWithContext with a
// *bytes.Reader or *bytes.Buffer body sets GetBody automatically).
var ErrBodyNotReplayable = errors.New("ratelimited: cannot retry request: body was consumed and GetBody is not set")

// Transport is an http.RoundTripper that adds pre-flight token-budget
// acquisition, automatic retry with Retry-After header parsing, and
// exponential backoff with jitter.
//
// # Order of operations on each attempt
//
//  1. Weight(req) — determine the token cost of this request.
//  2. Limiter.Wait(ctx, weight) — acquire budget before the attempt.
//  3. Base.RoundTrip(req) — issue the underlying HTTP call.
//  4. On any non-network response: ParseLimit(resp) → Limiter.Update(rem, resetAt)
//     if ParseLimit returns ok=true. This runs on 429s as well as 200s so the
//     bucket reflects the authoritative server state immediately.
//  5. On 429: parse Retry-After (delta-seconds or HTTP-date per RFC 7231).
//     Sleep for the indicated duration (or exponential backoff if absent),
//     then goto 1 for the next attempt.
//  6. On 502/503/504: exponential backoff + retry.
//  7. On network error (resp==nil, err!=nil): exponential backoff + retry.
//  8. On any other 4xx: return immediately — no retry.
//  9. Cap total attempts at MaxRetries+1. Cap total elapsed at MaxRetryElapsed
//     measured from the start of RoundTrip.
//
// # Body re-readability
//
// HTTP's RoundTripper contract allows the transport to consume req.Body on
// the first attempt. To retry requests with bodies, Transport calls
// req.GetBody() to obtain a fresh io.ReadCloser before each retry. If GetBody
// is nil and the request has a body, Transport returns ErrBodyNotReplayable on
// the first retry attempt.
//
// http.NewRequestWithContext automatically sets GetBody when the body is a
// *bytes.Reader, *bytes.Buffer, or string reader — no extra effort needed for
// the common case of JSON-marshalled request bodies.
//
// # Context cancellation
//
// ctx.Done() is always honoured. The Sleep function (defaulting to ctxSleep)
// selects on ctx.Done() and returns ctx.Err() immediately on cancellation,
// preventing the transport from sleeping through a cancelled context.
type Transport struct {
	// Base is the underlying RoundTripper. Defaults to http.DefaultTransport
	// when nil.
	Base http.RoundTripper

	// Limiter is the rate-limit budget shared across all RoundTrip calls on
	// this Transport. The field type is the driven.Limiter interface so that
	// tests can substitute lightweight fakes.
	Limiter driven.Limiter

	// Weight returns the token cost of a request. If nil, every request has
	// weight 1. The weight is passed to Limiter.Wait before the attempt.
	Weight func(*http.Request) int64

	// ParseLimit extracts the authoritative remaining-budget and reset-time
	// from a response. Called on every response that is not a network error.
	// If ParseLimit is nil or returns ok=false, Limiter.Update is not called.
	ParseLimit func(*http.Response) (remaining int64, resetAt time.Time, ok bool)

	// MaxRetries is the maximum number of retry attempts after the initial
	// attempt. A value of 0 means no retries (one total attempt). Negative
	// values are treated as 0. The OpenAI client constructors always set this
	// field explicitly from env-var or option; the zero-value does not apply
	// any built-in default.
	MaxRetries int

	// MaxRetryElapsed is the maximum total time to spend on retries, measured
	// from the start of the first RoundTrip call. Defaults to 60s.
	MaxRetryElapsed time.Duration

	// Clock returns the current time. Defaults to time.Now. Overridable for
	// tests.
	Clock func() time.Time

	// Sleep sleeps for d or until ctx is done. Returns ctx.Err() on
	// cancellation. Defaults to ctxSleep. Overridable for tests.
	Sleep func(ctx context.Context, d time.Duration) error
}

// base returns the Base transport, falling back to http.DefaultTransport.
func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

// clock returns the Clock function, falling back to time.Now.
func (t *Transport) clock() func() time.Time {
	if t.Clock != nil {
		return t.Clock
	}
	return time.Now
}

// sleep returns the Sleep function, falling back to ctxSleep.
func (t *Transport) sleep() func(ctx context.Context, d time.Duration) error {
	if t.Sleep != nil {
		return t.Sleep
	}
	return ctxSleep
}

// maxRetries returns MaxRetries unchanged. A value of 0 means no retries
// (only the initial attempt). Negative values are treated as 0.
// The OpenAI constructors always set this field explicitly from env or opts.
func (t *Transport) maxRetries() int {
	if t.MaxRetries < 0 {
		return 0
	}
	return t.MaxRetries
}

// maxRetryElapsed returns the effective MaxRetryElapsed, defaulting to 60s.
func (t *Transport) maxRetryElapsed() time.Duration {
	if t.MaxRetryElapsed > 0 {
		return t.MaxRetryElapsed
	}
	return 60 * time.Second
}

// weight returns the token cost of the request.
func (t *Transport) weight(req *http.Request) int64 {
	if t.Weight != nil {
		if w := t.Weight(req); w > 0 {
			return w
		}
	}
	return 1
}

// RoundTrip implements http.RoundTripper with retry and rate-limit logic.
// See the Transport godoc for a full description of the flow.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	start := t.clock()()
	deadline := start.Add(t.maxRetryElapsed())

	maxAttempts := t.maxRetries() + 1 // total attempts = retries + 1
	weight := t.weight(req)
	sleepFn := t.sleep()

	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check elapsed time before each attempt (after the first).
		if attempt > 0 && t.clock()().After(deadline) {
			return nil, fmt.Errorf("ratelimited: max retry elapsed (%s) exceeded: %w",
				t.maxRetryElapsed(), lastErr)
		}

		// Pre-flight budget acquisition.
		if t.Limiter != nil {
			if err := t.Limiter.Wait(ctx, weight); err != nil {
				return nil, err
			}
		}

		// Prepare body for this attempt.
		if attempt > 0 {
			if req.Body != nil || req.GetBody != nil {
				if req.GetBody == nil {
					return nil, ErrBodyNotReplayable
				}
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("ratelimited: GetBody failed on retry %d: %w", attempt, err)
				}
				req.Body = body
			}
		}

		resp, err := t.base().RoundTrip(req)

		if err != nil {
			// Network-level error — no response body to drain.
			lastErr = err

			isLastAttempt := attempt == maxAttempts-1
			if isLastAttempt {
				return nil, lastErr
			}

			backoff := t.backoffDuration(attempt)
			if sleepErr := sleepFn(ctx, backoff); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		// Non-network response: call ParseLimit on every response so the
		// bucket reflects the authoritative server state even on 429s.
		if t.ParseLimit != nil && t.Limiter != nil {
			if rem, resetAt, ok := t.ParseLimit(resp); ok {
				t.Limiter.Update(rem, resetAt)
			}
		}

		isLastAttempt := attempt == maxAttempts-1

		// Determine retry strategy based on status code.
		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			if isLastAttempt {
				// Return the final 429 response with body intact so the caller
				// can read the body-level error details (e.g. provider-specific
				// error codes in the JSON body).
				return resp, nil
			}
			// Drain and close the body before sleeping and retrying.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("ratelimited: server returned 429")

			delay := t.retryAfterDelay(resp, attempt)
			if sleepErr := sleepFn(ctx, delay); sleepErr != nil {
				return nil, sleepErr
			}

		case resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusGatewayTimeout:
			if isLastAttempt {
				// Return the final response so the caller sees the body.
				return resp, nil
			}
			// 5xx transient — drain and retry with backoff.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("ratelimited: server returned %d", resp.StatusCode)

			backoff := t.backoffDuration(attempt)
			if sleepErr := sleepFn(ctx, backoff); sleepErr != nil {
				return nil, sleepErr
			}

		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			// Non-retryable 4xx (401, 403, 404, etc.) — return immediately.
			return resp, nil

		default:
			// Success or other non-retryable status.
			return resp, nil
		}
	}

	// Should not be reached: the loop always returns on the last attempt.
	return nil, fmt.Errorf("ratelimited: max retries (%d) exhausted", t.maxRetries())
}

// retryAfterDelay returns the delay to wait before the next attempt, based on
// the Retry-After response header (RFC 7231 delta-seconds or HTTP-date) or
// exponential backoff with jitter if the header is absent.
func (t *Transport) retryAfterDelay(resp *http.Response, attempt int) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		// Try delta-seconds first.
		if secs, err := strconv.ParseFloat(ra, 64); err == nil && secs >= 0 {
			return time.Duration(secs * float64(time.Second))
		}
		// Try HTTP-date (net/http.ParseTime handles all RFC 7231 date formats).
		if t2, err := http.ParseTime(ra); err == nil {
			now := t.clock()()
			if d := t2.Sub(now); d > 0 {
				return d
			}
			return 0
		}
	}
	return t.backoffDuration(attempt)
}

// backoffDuration returns the exponential backoff duration for the given attempt
// number, with 0-25% random jitter added.
//
// Backoff sequence (before jitter): 1s, 2s, 4s, 8s, 16s, capped at 30s.
func (t *Transport) backoffDuration(attempt int) time.Duration {
	const maxBackoff = 30 * time.Second
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > maxBackoff {
		base = maxBackoff
	}
	// Add 0-25% jitter.
	jitter := time.Duration(rand.Float64() * 0.25 * float64(base))
	return base + jitter
}

// ctxSleep sleeps for d or returns ctx.Err() if ctx is done before d elapses.
func ctxSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// WeightFromHeader returns a Weight function that reads the token cost from the
// HeaderTokenWeight request header. The header value must be a base-10 integer.
// If the header is absent or malformed, the function returns 1.
//
// This is the recommended Weight function for OpenAI clients, which set the
// header before calling client.Do.
func WeightFromHeader(req *http.Request) int64 {
	v := req.Header.Get(HeaderTokenWeight)
	if v == "" {
		return 1
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 1
	}
	return n
}
