package driven

import "context"

// RetrievalObserver is invoked when a retrieval operation (search, document
// fetch) completes successfully. Implementations may perform arbitrary
// post-processing: metrics emission, structured logging, cache updates, audit
// writes, etc.
//
// Contract:
//   - Called after the response has been built, on a successful path only.
//     Not called for 4xx/5xx responses.
//   - Invoked asynchronously on a separate goroutine. Observers must not
//     assume the originating request context is still live; the ctx passed in
//     is detached from the request.
//   - Returned errors are logged and ignored — observer failure does NOT
//     affect the response returned to the caller.
//
// This is the retrieval-side mirror of DocumentIngestObserver. The key
// difference is that retrieval observers run asynchronously: they sit on the
// user-facing request path, so observer latency must not add to response
// time. Ingest observers run synchronously because they sit on a background
// sync path where latency is not user-visible.
type RetrievalObserver interface {
	OnSearchCompleted(ctx context.Context, event SearchCompletedEvent) error
	OnDocumentRetrieved(ctx context.Context, event DocumentRetrievedEvent) error
}

// SearchCompletedEvent carries the outcome of a search request.
//
// ClientID and ClientName identify the calling application (e.g. an OAuth
// client such as Claude Desktop or Cursor), not the end user. They are
// optional: adapters where application-level identity doesn't apply (such
// as session-based HTTP auth) leave both empty. UserID still identifies
// the human; ClientType still identifies the transport.
type SearchCompletedEvent struct {
	UserID      string
	Query       string
	DocumentIDs []string
	ResultCount int
	DurationNs  int64
	ClientType  string // "http" | "mcp" | etc.
	ClientID    string // OAuth client_id for MCP; empty when not applicable.
	ClientName  string // Human-readable client name; empty when not known.
}

// DocumentRetrievedEvent carries the outcome of a single-document fetch.
//
// ClientID and ClientName follow the same semantics as on
// SearchCompletedEvent: they identify the calling application and are
// optional — empty is valid whenever the concept doesn't apply.
type DocumentRetrievedEvent struct {
	UserID     string
	DocumentID string
	DurationNs int64
	ClientType string
	ClientID   string
	ClientName string
}
