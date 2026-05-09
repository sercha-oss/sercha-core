package driven

import "context"

// ExtractionContextStore exposes a single admin-editable string that the
// entity-extractor stage appends to its LLM prompt as additional domain
// context (e.g. "this corpus contains insurance case files; CL-NNNN-NNNN
// patterns are claim numbers"). The store backs a singleton row — the
// taxonomy of categories is governed elsewhere; this is purely a free-form
// instructional bias for the model.
//
// Get returns the current context string. An empty string is the
// default-installed value and means "no additional context"; callers must
// be tolerant of empty strings.
//
// Set replaces the singleton row's content. Implementations must be
// idempotent and safe under concurrent calls (last writer wins).
type ExtractionContextStore interface {
	Get(ctx context.Context) (string, error)
	Set(ctx context.Context, context string) error
}
