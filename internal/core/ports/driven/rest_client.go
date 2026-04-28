package driven

import (
	"context"
	"errors"
)

// ErrRESTUnsupported is returned by RESTClient implementations attached to
// connectors that do not speak HTTP — the local filesystem connector is the
// canonical example. Callers should treat this as a hard signal that the
// connector is not a valid target for the operation and bail out rather
// than retry.
var ErrRESTUnsupported = errors.New("connector does not expose a REST client")

// RESTClient is the per-connector seam for callers that need to invoke
// adapter-side REST endpoints not covered by the typed methods on a
// Connector. Use it when you want to extend a connector with an additional
// upstream API call (e.g. an admin or metadata endpoint) without
// reimplementing the connector's auth, rate-limiting, and retry plumbing.
//
// Implementations MUST:
//   - apply the connector's existing auth (token from the wired
//     TokenProvider), rate-limiting, and retry/backoff semantics — Do is a
//     thin wrapper over the adapter's existing HTTP plumbing, not a parallel
//     code path that bypasses it.
//   - JSON-encode the body if non-nil, JSON-decode the response into result
//     if non-nil. Implementations that cannot do this for a particular
//     endpoint (e.g. binary downloads) should expose a typed method on the
//     concrete adapter instead of routing through Do.
//   - return a typed error for known recoverable conditions (e.g. cursor
//     expiry); raw network errors are returned wrapped.
//
// Connectors with no REST surface return a sentinel implementation whose Do
// always returns ErrRESTUnsupported, so callers can rely on
// Connector.RESTClient() being non-nil.
type RESTClient interface {
	// Do executes an authenticated, rate-limited, retried request against
	// path (a connector-relative path; the adapter applies its own base URL)
	// and decodes the JSON response into result. body, if non-nil, is
	// JSON-encoded as the request body.
	//
	// Callers MUST NOT pass absolute URLs in path.
	Do(ctx context.Context, method, path string, body, result any) error
}
