package indexing

import (
	"context"
	"log/slog"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/entitydetect"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const EntityExtractorStageID = "entity-extractor"

// detectionTimeout bounds the time the LLM work may take per document.
// With chunked extraction AND proper TPM/RPM throttling, a single document
// under contention can legitimately spend several minutes waiting on bucket
// refills before its chunks complete. The budget is generous enough that a
// 50-page PDF behind a busy queue still finishes, but bounded so a stuck
// call can't pin an indexing worker forever. On timeout any partial result
// is cached and the stage logs a warning.
const detectionTimeout = 5 * time.Minute

// EntityExtractorFactory creates the indexing-time entity-extractor stage.
//
// The factory holds the EntityRegister, EntityTypeRegistry, and the
// optional ExtractionContextStore as constructor args, not capabilities.
// They are application-internal ports backed by the app's own database —
// the same shape as DocumentStore — so they're wired directly in main.go
// and threaded through here. The capability registry is reserved for
// external swappable services (LLM provider, search engine, vector store).
// maxInFlightLLMCalls bounds the total number of concurrent DetectChunked
// invocations across the whole process. The upstream sync orchestrator
// multiplies worker concurrency × per-container doc fan-out, which can
// produce far more parallel demand than the LLM provider's TPM bucket can
// serve. Without this gate the bucket queue grows past the http client's
// deadline and rate.Limiter.WaitN starts rejecting requests.
//
// Set to 8 — sized for tier-1 gpt-4o-mini at LLM_TPM=200000 with
// chunkSizeBytes=96000 (~24K tokens per request). Eight in-flight 24K-token
// requests is 192K tokens of demand against the 200K TPM bucket; the
// bucket can refill that in ~58s steady-state, leaving the 5-minute
// per-request http timeout with comfortable headroom even if all eight
// queue simultaneously. Operators on higher tiers can raise this — the
// bucket itself remains the authoritative gate.
const maxInFlightLLMCalls = 8

type EntityExtractorFactory struct {
	descriptor   pipeline.StageDescriptor
	register     driven.EntityRegister
	registry     pipelineport.EntityTypeRegistry
	contextStore driven.ExtractionContextStore
	// llmSem caps in-flight DetectChunked calls. Buffered channel pattern;
	// send before invoking the LLM, receive when done. Shared across every
	// stage instance the factory creates so the cap is process-wide, not
	// per-instance.
	llmSem chan struct{}
}

// NewEntityExtractorFactory creates a new indexing-time entity-extractor
// factory.
//
// register is the per-document analysis cache. May be nil — the stage will
// run detection without caching when nil (only useful for tests).
//
// registry is the entity-type taxonomy. May be nil — the stage falls back to
// a minimal built-in list. Production wiring always passes a non-nil registry.
//
// contextStore supplies the optional admin-editable extra prompt context.
// May be nil — the stage falls back to no extra context, matching the
// default-empty install state.
func NewEntityExtractorFactory(register driven.EntityRegister, registry pipelineport.EntityTypeRegistry, contextStore driven.ExtractionContextStore) *EntityExtractorFactory {
	return &EntityExtractorFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          EntityExtractorStageID,
			Name:        "Entity Extractor",
			Type:        pipeline.StageTypeEnricher,
			InputShape:  pipeline.ShapeContent,
			OutputShape: pipeline.ShapeContent,
			Cardinality: pipeline.CardinalityOneToOne,
			Capabilities: []pipeline.CapabilityRequirement{
				// Only the LLM is a capability. Register and registry are
				// application-internal CRUD ports passed via the constructor.
				{Type: pipeline.CapabilityLLM, Mode: pipeline.CapabilityOptional},
			},
			Version: "1.0.0",
		},
		register:     register,
		registry:     registry,
		contextStore: contextStore,
		llmSem:       make(chan struct{}, maxInFlightLLMCalls),
	}
}

// StageID returns the stage identifier.
func (f *EntityExtractorFactory) StageID() string { return f.descriptor.ID }

// Descriptor returns the stage descriptor.
func (f *EntityExtractorFactory) Descriptor() pipeline.StageDescriptor { return f.descriptor }

// Validate validates the stage configuration. No config is required.
func (f *EntityExtractorFactory) Validate(config pipeline.StageConfig) error { return nil }

// Create creates a new entity-extractor stage. The LLM is pulled optionally
// from the capability set; the register and registry come from the factory.
func (f *EntityExtractorFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	var llm driven.LLMService
	if inst, ok := capabilities.Get(pipeline.CapabilityLLM); ok {
		if l, ok := inst.Instance.(driven.LLMService); ok {
			llm = l
		}
	}
	return &EntityExtractorStage{
		descriptor:   f.descriptor,
		llm:          llm,
		register:     f.register,
		registry:     f.registry,
		contextStore: f.contextStore,
		llmSem:       f.llmSem,
		logger:       slog.Default(),
	}, nil
}

// EntityExtractorStage runs LLM-driven entity detection at indexing time and
// writes the validated spans to the entity_register cache. Downstream search
// reads the cache via the entity-retriever stage.
//
// The stage is a pass-through on the IndexingInput value — the indexing
// pipeline shape is preserved so chunker/embedder/vector-loader continue to
// run unchanged after entity extraction.
//
// Fail-soft contract: any LLM, register, or taxonomy error is logged at warn
// level and the input is passed through untouched. Indexing must not be
// gated on the success of entity detection.
type EntityExtractorStage struct {
	descriptor   pipeline.StageDescriptor
	llm          driven.LLMService
	register     driven.EntityRegister
	registry     pipelineport.EntityTypeRegistry
	contextStore driven.ExtractionContextStore
	llmSem       chan struct{}
	logger       *slog.Logger
}

// Descriptor returns the stage descriptor.
func (s *EntityExtractorStage) Descriptor() pipeline.StageDescriptor { return s.descriptor }

// Process detects entities in the document content and writes the validated
// spans to the entity_register cache.
func (s *EntityExtractorStage) Process(ctx context.Context, input any) (any, error) {
	procStart := time.Now()
	indexInput, ok := input.(*pipeline.IndexingInput)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.IndexingInput"}
	}

	// Graceful no-op when the LLM isn't wired (Core build, tests).
	if s.llm == nil {
		return indexInput, nil
	}

	// Skip empty bodies — chunker/embedder will likewise skip them, and
	// caching an empty span list serves no purpose.
	if indexInput.Content == "" {
		return indexInput, nil
	}

	categories, ok := s.loadCategories(ctx)
	if !ok {
		// Registry unavailable or empty — skip detection entirely. The
		// document still flows through chunker/embedder/vector-loader so
		// search will still work, just without entity metadata.
		return indexInput, nil
	}
	// Load the admin-supplied prompt context up-front so it contributes to
	// the cache key. Empty string is the default; any error degrades to no
	// extra context (and the cache key reflects that, so we don't accidentally
	// treat an "errors-by-default" run as cache-equivalent to a real run).
	extraContext := ""
	if s.contextStore != nil {
		if c, err := s.contextStore.Get(ctx); err != nil {
			s.logger.Warn("entity-extractor: extraction context fetch failed; proceeding without",
				"stage", s.descriptor.ID,
				"document_id", indexInput.DocumentID,
				"error", err,
			)
		} else {
			extraContext = c
		}
	}
	analyzerVersion := entitydetect.AnalyzerVersion(categories, extraContext)
	contentSHA := entitydetect.ContentSHA256(indexInput.Content)

	// Cache hit: the same content was already analyzed at this analyzer
	// version. Skip the LLM call entirely.
	if s.register != nil {
		if _, hit, err := s.register.Get(ctx, indexInput.DocumentID, contentSHA, analyzerVersion); err != nil {
			s.logger.Warn("entity-extractor: register.Get failed; treating as miss",
				"stage", s.descriptor.ID,
				"document_id", indexInput.DocumentID,
				"error", err,
			)
		} else if hit {
			return indexInput, nil
		}
	}

	// Acquire the in-flight LLM-call slot before kicking off detection. This
	// is in addition to the rate-limited transport's own bucket — without
	// it the bucket queue grows unboundedly when upstream concurrency is
	// higher than the bucket can serve, eventually causing
	// rate.Limiter.WaitN to reject requests as "would exceed deadline".
	semWaitStart := time.Now()
	if s.llmSem != nil {
		select {
		case s.llmSem <- struct{}{}:
		case <-ctx.Done():
			return indexInput, nil // pass-through; ctx cancel is normal at shutdown
		}
		defer func() { <-s.llmSem }()
	}
	semWaitMs := time.Since(semWaitStart).Milliseconds()

	dctx, cancel := context.WithTimeout(ctx, detectionTimeout)
	defer cancel()

	// DetectChunked iterates over chunks of the document, each LLM call
	// seeded with the entities found by previous chunks. The returned
	// slice is already validated against the full content (offsets,
	// hallucination guard, dedup).
	detectStart := time.Now()
	validated, err := entitydetect.DetectChunked(dctx, s.llm, indexInput.Content, categories, extraContext)
	detectMs := time.Since(detectStart).Milliseconds()
	if err != nil {
		// On any chunk error: log and pass the document through WITHOUT
		// writing to the cache. The rest of the indexing pipeline (chunker,
		// embedder, vector-loader) still runs so the doc remains searchable;
		// the next sync will see no v3 cache entry for this document and
		// retry detection. Partial caching was a regression source — once
		// a doc was cached with empty/partial spans, the cache hit on the
		// next sync skipped the LLM call and the doc stayed broken forever.
		s.logger.Warn("entity-extractor: chunked detect failed; spans not cached, will retry on next sync",
			"stage", s.descriptor.ID,
			"document_id", indexInput.DocumentID,
			"partial_spans", len(validated),
			"error", err,
		)
		return indexInput, nil
	}

	if s.register != nil {
		analysis := &driven.EntityAnalysis{
			DocumentID:      indexInput.DocumentID,
			ContentSHA256:   contentSHA,
			AnalyzerVersion: analyzerVersion,
			RulesetVersion:  1,
			Spans:           validated,
			AnalyzedAt:      time.Now().Unix(),
		}
		if err := s.register.Put(ctx, analysis); err != nil {
			s.logger.Warn("entity-extractor: register.Put failed; spans not cached",
				"stage", s.descriptor.ID,
				"document_id", indexInput.DocumentID,
				"error", err,
			)
		}
	}

	s.logger.Info("entity-extractor: doc complete",
		"document_id", indexInput.DocumentID,
		"doc_bytes", len(indexInput.Content),
		"sem_wait_ms", semWaitMs,
		"detect_ms", detectMs,
		"total_ms", time.Since(procStart).Milliseconds(),
		"spans", len(validated),
	)
	return indexInput, nil
}

// loadCategories reads the active taxonomy from the registry. Returns
// (categories, true) when a non-empty list is available; returns
// (nil, false) for any of: no registry wired, registry call failed, or
// the admin has emptied the taxonomy.
//
// On (nil, false) the caller skips detection for this document. We do not
// fall back to a hard-coded list: a transient DB error would silently
// produce stale results that hash to a stable cache key and never
// auto-recover; an empty admin-curated list is a deliberate policy choice
// the stage must respect.
func (s *EntityExtractorStage) loadCategories(ctx context.Context) ([]pipeline.EntityTypeMetadata, bool) {
	if s.registry == nil {
		return nil, false
	}
	types, err := s.registry.List(ctx)
	if err != nil {
		s.logger.Warn("entity-extractor: registry.List failed; skipping detection for this document",
			"stage", s.descriptor.ID,
			"error", err,
		)
		return nil, false
	}
	if len(types) == 0 {
		return nil, false
	}
	return types, true
}

// Compile-time interface assertions.
var _ pipelineport.StageFactory = (*EntityExtractorFactory)(nil)
var _ pipelineport.Stage = (*EntityExtractorStage)(nil)
