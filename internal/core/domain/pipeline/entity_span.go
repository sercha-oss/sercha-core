package pipeline

// EntitySpan represents a single detected entity within a piece of text.
//
// Start and End are byte offsets into the source text the span was detected in
// (not Unicode codepoints). Start is inclusive; End is exclusive, so the span
// covers text[Start:End]. Value is the exact substring as it appears in the
// source — consumers must not assume it is normalised or trimmed.
//
// The entity-extractor stage does not trust offsets reported by a detector.
// Instead it derives Start and End server-side via strings.Index against Value.
// Spans whose Value is not a substring of the source text are dropped.
type EntitySpan struct {
	// Type is the entity category, matched against the registered taxonomy.
	Type EntityType `json:"type"`

	// Value is the exact substring of the source text that was detected.
	Value string `json:"value"`

	// Start is the inclusive byte offset of the span in the source text.
	Start int `json:"start"`

	// End is the exclusive byte offset of the span in the source text.
	End int `json:"end"`

	// Confidence is the detector's confidence score in [0.0, 1.0].
	Confidence float64 `json:"confidence"`

	// Detector is the identity of the detector that produced this span.
	Detector string `json:"detector"`
}
