// Package domain defines the core domain types for the Sercha search platform.
//
// Caller represents request-source identity: it is constructed at every driving
// entry point (HTTP search handler, MCP search handler, MCP get_document handler)
// and threaded through SearchContext so that pipeline stages which branch on
// caller type — for example, to apply policy only to a specific request
// origin — have a stable, well-typed signal to read.
package domain

// CallerSource identifies the entry point through which a search request arrived.
// It is an open string type following the same convention as SearchMode — a small
// fixed set of constants covers the currently known values, but consumers should
// not panic on unrecognised values.
type CallerSource string

const (
	// CallerSourceDirect indicates the request arrived via the direct HTTP search
	// API (the UI or REST API consumer).
	CallerSourceDirect CallerSource = "direct"

	// CallerSourceMCP indicates the request arrived from an OAuth2 LLM client via
	// the Model Context Protocol server.
	CallerSourceMCP CallerSource = "mcp"
)

// Caller carries the request-source identity for a single pipeline execution.
//
// This is the base form understood by Core. Consumers may extend it with
// additional metadata (e.g. OAuth2 client identity, delegation mode) by
// embedding this struct in a richer type and threading the richer value
// through the request context — see the CallerEnricher hook on the MCP
// adapter for the canonical pattern.
//
// The UserID field mirrors SearchContext.UserID — it is duplicated here so that
// stages receiving only the Caller have all the identity they need without an
// extra context lookup.
type Caller struct {
	// Source identifies the entry point that constructed this Caller.
	Source CallerSource `json:"source"`

	// UserID is the authenticated user performing the request.
	// Empty string signals an unauthenticated or service-to-service call.
	UserID string `json:"user_id"`
}
