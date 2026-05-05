// Package ratelimited provides a generic rate-limited HTTP transport adapter
// and a token-bucket implementation of the driven.Limiter port.
//
// The two central types are:
//
//   - Bucket: a concrete Limiter implementation that tracks a token budget
//     and supports server-authoritative reconciliation via Update. Backed by
//     golang.org/x/time/rate for refill timing, with a mutex-guarded remaining
//     counter layered on top for Update semantics.
//
//   - Transport: an http.RoundTripper wrapper that adds pre-flight budget
//     acquisition, automatic retry with Retry-After parsing, exponential
//     backoff, and per-provider response-header callbacks. Any HTTP client
//     can plug in a Transport to gain consistent retry and rate-limit
//     behaviour without duplicating the logic per provider.
//
// The two types compose naturally: construct a Bucket, pass it to a Transport,
// and the transport acquires budget before each attempt and reconciles
// authoritative server state from response headers after each attempt.
package ratelimited
