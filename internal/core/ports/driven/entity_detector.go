package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// EntityDetector extracts named-entity spans from a piece of text.
// Implementations may be backed by an LLM, a local model, a regex ruleset, or
// any other mechanism.
type EntityDetector interface {
	// Detect runs entity detection on text and returns the discovered spans.
	//
	// Implementations should return spans whose Value field is an exact
	// substring of text. The consuming entity-extractor stage validates this
	// contract: any span whose Value is not found in the source text is silently
	// dropped before the results are attached to a candidate.
	//
	// Implementations are free to populate Start and End for their own
	// convenience, but the consuming stage does not trust detector-reported
	// offsets. It derives Start and End server-side via strings.Index against
	// Value. This guards against hallucinated offsets from LLM-backed detectors
	// and against serialisation quirks in other implementations.
	//
	// Errors returned from Detect are treated as fail-soft by the consuming
	// stage: the error is logged at warn level, the candidate is skipped (no
	// spans attached), and processing continues with the remaining candidates.
	// Implementations should not return a partial span slice alongside an error;
	// return either (spans, nil) or (nil, err).
	Detect(ctx context.Context, text string) ([]pipeline.EntitySpan, error)

	// OwnedCategories declares the subset of the active entity taxonomy that
	// this detector handles.
	//
	// The return value is used for partition validation when multiple detectors
	// are configured: each entity category should be owned by exactly one
	// detector. If two registered detectors both claim the same category,
	// EntityTypeRegistry.SetOwningDetector will return an owner-conflict error
	// for the second claimant.
	//
	// Returning an empty slice means "this detector owns nothing", which is
	// rare but valid. An implementation that returns an empty slice will never
	// trigger an owner-conflict error during registration.
	//
	// OwnedCategories must be a pure, cheap read. It is called during detector
	// registration and must not block on I/O.
	OwnedCategories() []pipeline.EntityType
}
