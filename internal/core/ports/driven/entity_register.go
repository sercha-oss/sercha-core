package driven

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
)

// EntityAnalysis is the cached result of running entity detection over a
// specific version of a document's content. It carries enough information to
// decide whether a stored result is still valid for the current document and
// detector configuration.
type EntityAnalysis struct {
	// DocumentID identifies the document this analysis belongs to.
	DocumentID string `json:"document_id"`

	// ContentSHA256 is the hex-encoded SHA-256 digest of the document content
	// at the time this analysis was produced. Together with AnalyzerVersion it
	// forms the cache validity key: a stored row is a hit only when both fields
	// match the incoming request.
	ContentSHA256 string `json:"content_sha256"`

	// AnalyzerVersion is an opaque token that identifies the detector
	// configuration used to produce this analysis. Changing the detector or its
	// configuration should produce a new AnalyzerVersion so that stale cached
	// results are not served.
	AnalyzerVersion string `json:"analyzer_version"`

	// RulesetVersion records the taxonomy version in effect when this analysis
	// was produced. Reserved for future taxonomy-evolution invalidation; the
	// entity-extractor stage does not currently use it to invalidate cached rows.
	RulesetVersion int `json:"ruleset_version"`

	// Spans contains the entity spans detected in the document content.
	Spans []pipeline.EntitySpan `json:"spans"`

	// AnalyzedAt is the Unix timestamp (seconds) at which this analysis was
	// produced and stored.
	AnalyzedAt int64 `json:"analyzed_at"`
}

// EntityRegister is a cache for entity analysis results, keyed on a combination
// of document ID, content digest, and analyzer version. It allows the
// entity-extractor stage to skip re-detection when the document content and
// detector configuration have not changed since the last analysis.
type EntityRegister interface {
	// Get retrieves a cached EntityAnalysis for the given document, content
	// digest, and analyzer version.
	//
	// It applies a three-case contract on the return:
	//
	//   - hit (*EntityAnalysis, true, nil): the store contains a row whose
	//     doc_id, content_sha256, AND analyzer_version all match the request.
	//     The returned analysis may be used directly; the consuming stage must
	//     not re-run detection.
	//
	//   - miss (nil, false, nil): the store either has no row for doc_id, or
	//     has a row whose content_sha256 or analyzer_version does not match.
	//     Both cases are misses, not errors. The consuming stage should fall
	//     through to the detector. Implementations must not return a non-nil
	//     error to signal a mismatch.
	//
	//   - storage error (nil, false, non-nil error): a problem with the
	//     underlying store prevented the read. The consuming stage treats this
	//     the same as a miss — it falls through to the detector — but it logs
	//     the error at warn level so operators can observe store degradation.
	//
	// The content_sha256 and analyzer_version arguments are the values computed
	// by the consuming stage for the current request; implementations must not
	// derive them independently.
	Get(ctx context.Context, docID, contentSHA256, analyzerVersion string) (*EntityAnalysis, bool, error)

	// Put stores an EntityAnalysis, keyed on (doc_id, analyzer_version).
	//
	// Put is an upsert: if a row with the same (doc_id, analyzer_version) key
	// already exists it is overwritten. This allows a content change (new
	// content_sha256) to invalidate the previous result for the same document
	// and analyzer version.
	//
	// Callers treat Put as best-effort: the consuming stage logs a warn on
	// error but continues to attach the freshly-detected spans to the candidate
	// regardless of whether caching succeeded.
	Put(ctx context.Context, analysis *EntityAnalysis) error
}
