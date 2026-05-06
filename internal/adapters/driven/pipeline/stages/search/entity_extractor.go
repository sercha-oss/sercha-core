// Package search contains the search pipeline stage adapters.
//
// # Entity extractor metadata key contract
//
// The entity-extractor stage attaches two keys to each candidate's Metadata map.
// Downstream consumers (ranker, presenter, enterprise ACL adapters) may rely on
// these keys being present whenever detection runs successfully for a candidate:
//
//   - "entities"        → []pipeline.EntitySpan
//     Validated spans sorted by Start (ascending byte offset). The slice is always
//     non-nil when written; it may be empty if all spans fail substring validation.
//     The key is absent when the detector is nil (graceful no-op path) or when
//     detection fails for a specific candidate (fail-soft: that candidate is
//     skipped entirely, others proceed).
//
//   - "entity_summary"  → map[pipeline.EntityType]int
//     Count of validated spans per entity type. Always written alongside
//     "entities" and always consistent with it. May be an empty map when no spans
//     survived validation.
//
// Both keys are always set together: if "entities" is present so is
// "entity_summary", and vice versa. Consumers must check for key presence; the
// keys are absent for any candidate the stage skips (nil detector or detection
// error for that candidate).
package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const EntityExtractorStageID = "entity-extractor"

// EntityExtractorFactory creates entity extractor stages.
type EntityExtractorFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewEntityExtractorFactory creates a new entity extractor factory.
func NewEntityExtractorFactory() *EntityExtractorFactory {
	return &EntityExtractorFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          EntityExtractorStageID,
			Name:        "Entity Extractor",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeCandidate,
			OutputShape: pipeline.ShapeCandidate,
			Cardinality: pipeline.CardinalityOneToOne,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityEntityDetector, Mode: pipeline.CapabilityOptional},
				{Type: pipeline.CapabilityEntityRegister, Mode: pipeline.CapabilityOptional},
				{Type: pipeline.CapabilityEntityTypeRegistry, Mode: pipeline.CapabilityOptional},
			},
			Version: "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *EntityExtractorFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *EntityExtractorFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Validate validates the stage configuration. No config is required for MVP.
func (f *EntityExtractorFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// Create creates a new entity extractor stage. Each capability is pulled
// optionally: a missing capability results in a nil field on the stage.
// The stage is nil-safe and degrades gracefully when any capability is absent.
func (f *EntityExtractorFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	var detector driven.EntityDetector
	if inst, ok := capabilities.Get(pipeline.CapabilityEntityDetector); ok {
		if d, ok := inst.Instance.(driven.EntityDetector); ok {
			detector = d
		}
	}

	var register driven.EntityRegister
	if inst, ok := capabilities.Get(pipeline.CapabilityEntityRegister); ok {
		if r, ok := inst.Instance.(driven.EntityRegister); ok {
			register = r
		}
	}

	var registry pipelineport.EntityTypeRegistry
	if inst, ok := capabilities.Get(pipeline.CapabilityEntityTypeRegistry); ok {
		if r, ok := inst.Instance.(pipelineport.EntityTypeRegistry); ok {
			registry = r
		}
	}

	return &EntityExtractorStage{
		descriptor: f.descriptor,
		detector:   detector,
		register:   register,
		registry:   registry,
		logger:     slog.Default(),
	}, nil
}

// EntityExtractorStage annotates search candidates with named-entity spans
// detected from each candidate's content.
//
// All three capability fields (detector, register, registry) are optional and
// may be nil. When detector is nil, the stage is a pure pass-through. When
// register is nil, no cache lookups or writes are attempted. When registry is
// nil, span types are not validated against the taxonomy.
//
// Metadata keys written per candidate:
//   - "entities"       → []pipeline.EntitySpan  (sorted by Start)
//   - "entity_summary" → map[pipeline.EntityType]int
type EntityExtractorStage struct {
	descriptor pipeline.StageDescriptor
	detector   driven.EntityDetector
	register   driven.EntityRegister
	registry   pipelineport.EntityTypeRegistry
	logger     *slog.Logger
}

// Descriptor returns the stage descriptor.
func (s *EntityExtractorStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process annotates each candidate with entity spans.
//
// Fail-soft contract: an error from the detector or register for a single
// candidate is logged at warn level and that candidate is skipped; other
// candidates continue processing. The stage never fails the entire pipeline
// due to a per-candidate detection or cache error.
func (s *EntityExtractorStage) Process(ctx context.Context, input any) (any, error) {
	candidates, ok := input.([]*pipeline.Candidate)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected []*pipeline.Candidate"}
	}

	// Graceful no-op: if no detector is available, return input unchanged.
	if s.detector == nil {
		return candidates, nil
	}

	// analyzerVersion is an opaque token that identifies the detector
	// configuration used to produce an analysis. For MVP this is a fixed
	// string; future work will derive it from the detector's config so that
	// changing the detector invalidates cached results automatically.
	const analyzerVersion = "entity-detector-v1"

	for _, candidate := range candidates {
		s.processCandidate(ctx, candidate, analyzerVersion)
	}

	return candidates, nil
}

// processCandidate runs entity detection (or cache lookup) for a single
// candidate and attaches the validated spans to candidate.Metadata.
// All errors are handled fail-soft: a per-candidate error is logged and the
// candidate is skipped (no metadata attached).
func (s *EntityExtractorStage) processCandidate(ctx context.Context, candidate *pipeline.Candidate, analyzerVersion string) {
	// Compute the content digest used as part of the cache key.
	sumBytes := sha256.Sum256([]byte(candidate.Content))
	contentSHA := hex.EncodeToString(sumBytes[:])

	var spans []pipeline.EntitySpan
	cacheHit := false

	// Attempt a cache lookup when a register is available.
	if s.register != nil {
		analysis, hit, err := s.register.Get(ctx, candidate.DocumentID, contentSHA, analyzerVersion)
		if err != nil {
			s.logger.Warn("entity-extractor: register.Get failed; treating as miss",
				"stage", s.descriptor.ID,
				"document_id", candidate.DocumentID,
				"error", err,
			)
			// fall through to detector
		} else if hit {
			spans = analysis.Spans
			cacheHit = true
		}
	}

	// On cache miss (or no register), run the detector.
	if !cacheHit {
		detected, err := s.detector.Detect(ctx, candidate.Content)
		if err != nil {
			s.logger.Warn("entity-extractor: detector.Detect failed; skipping candidate",
				"stage", s.descriptor.ID,
				"document_id", candidate.DocumentID,
				"error", err,
			)
			return // fail-soft: skip this candidate
		}
		spans = detected
	}

	// Validate spans: drop any whose Value is not a substring of the content,
	// and recompute Start/End server-side for survivors. This guards against
	// hallucinated offsets from LLM-backed detectors and byte-vs-rune confusion.
	validated := make([]pipeline.EntitySpan, 0, len(spans))
	for i := range spans {
		span := spans[i]
		if !strings.Contains(candidate.Content, span.Value) {
			s.logger.Debug("entity-extractor: dropped hallucinated span",
				"stage", s.descriptor.ID,
				"document_id", candidate.DocumentID,
				"span_value", span.Value,
				"span_type", span.Type,
			)
			continue
		}
		idx := strings.Index(candidate.Content, span.Value)
		span.Start = idx
		span.End = idx + len(span.Value)

		// MVP: The entity-type registry is intentionally NOT consulted here to
		// validate span types. All detected span types are kept regardless of
		// whether they appear in the registered taxonomy. This is a deliberate
		// permissive stance for the initial release: taxonomy coverage may not
		// yet reflect every type a detector can produce, and silent drops would
		// be confusing during bring-up. A future PR that hardens type validation
		// can change this block — document the change so it is always intentional.
		// s.registry is retained on the stage struct for when that hardening lands.

		validated = append(validated, span)
	}

	// Sort validated spans by Start offset (ascending).
	sort.Slice(validated, func(i, j int) bool {
		return validated[i].Start < validated[j].Start
	})

	// Store the freshly-detected (and now validated) spans in the register
	// when we ran the detector (not a cache hit). Caching is best-effort:
	// a Put error is logged but does not prevent the spans from being attached.
	if !cacheHit && s.register != nil {
		analysis := &driven.EntityAnalysis{
			DocumentID:      candidate.DocumentID,
			ContentSHA256:   contentSHA,
			AnalyzerVersion: analyzerVersion,
			RulesetVersion:  1,
			Spans:           validated,
			AnalyzedAt:      time.Now().Unix(),
		}
		if err := s.register.Put(ctx, analysis); err != nil {
			s.logger.Warn("entity-extractor: register.Put failed; spans still attached (best-effort cache)",
				"stage", s.descriptor.ID,
				"document_id", candidate.DocumentID,
				"error", err,
			)
		}
	}

	// Build the per-type count summary over validated spans.
	summary := make(map[pipeline.EntityType]int, len(validated))
	for _, sp := range validated {
		summary[sp.Type]++
	}

	// Attach both metadata keys. Allocate the map if needed.
	if candidate.Metadata == nil {
		candidate.Metadata = make(map[string]any)
	}
	candidate.Metadata["entities"] = validated
	candidate.Metadata["entity_summary"] = summary
}

// Compile-time interface assertions.
var _ pipelineport.StageFactory = (*EntityExtractorFactory)(nil)
var _ pipelineport.Stage = (*EntityExtractorStage)(nil)
