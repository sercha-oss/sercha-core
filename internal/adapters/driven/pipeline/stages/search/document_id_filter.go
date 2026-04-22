package search

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

const DocumentIDFilterStageID = "document-id-filter"

// DocumentIDFilterFactory creates document ID filter stages.
type DocumentIDFilterFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewDocumentIDFilterFactory creates a new document ID filter factory.
func NewDocumentIDFilterFactory() *DocumentIDFilterFactory {
	return &DocumentIDFilterFactory{
		descriptor: pipeline.StageDescriptor{
			ID:          DocumentIDFilterStageID,
			Name:        "Document ID Filter",
			Type:        pipeline.StageTypeParser,
			InputShape:  pipeline.ShapeParsedQuery,
			OutputShape: pipeline.ShapeParsedQuery,
			Cardinality: pipeline.CardinalityOneToOne,
			Capabilities: []pipeline.CapabilityRequirement{
				{Type: pipeline.CapabilityDocumentIDProvider, Mode: pipeline.CapabilityOptional},
			},
			Version: "1.0.0",
		},
	}
}

// StageID returns the stage identifier.
func (f *DocumentIDFilterFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *DocumentIDFilterFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Create creates a new document ID filter stage.
func (f *DocumentIDFilterFactory) Create(config pipeline.StageConfig, capabilities *pipeline.CapabilitySet) (pipelineport.Stage, error) {
	// Check if DocumentIDProvider capability is available (it's optional)
	var provider driven.DocumentIDProvider
	if inst, ok := capabilities.Get(pipeline.CapabilityDocumentIDProvider); ok {
		if p, ok := inst.Instance.(driven.DocumentIDProvider); ok {
			provider = p
		}
	}

	return &DocumentIDFilterStage{
		descriptor: f.descriptor,
		provider:   provider,
	}, nil
}

// Validate validates the stage configuration.
func (f *DocumentIDFilterFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// DocumentIDFilterStage populates SearchFilters.DocumentIDFilter using a DocumentIDProvider.
// If no provider is available, the stage passes through and leaves DocumentIDFilter unset (nil).
type DocumentIDFilterStage struct {
	descriptor pipeline.StageDescriptor
	provider   driven.DocumentIDProvider
}

// Descriptor returns the stage descriptor.
func (s *DocumentIDFilterStage) Descriptor() pipeline.StageDescriptor {
	return s.descriptor
}

// Process translates the provider's three-case return into *domain.DocumentIDFilter on
// ParsedQuery.SearchFilters. A nil return leaves DocumentIDFilter untouched (no-op);
// a non-nil return — including an empty slice — constructs an authoritative filter.
func (s *DocumentIDFilterStage) Process(ctx context.Context, input any) (any, error) {
	parsed, ok := input.(*pipeline.ParsedQuery)
	if !ok {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "expected *pipeline.ParsedQuery"}
	}

	// If no provider is available, pass through without filtering
	if s.provider == nil {
		return parsed, nil
	}

	// Get allowed document IDs from the provider
	allowedIDs, err := s.provider.GetAllowedDocumentIDs(ctx, parsed.Original, parsed.SearchFilters)
	if err != nil {
		return nil, &StageError{Stage: s.descriptor.ID, Message: "failed to get allowed document IDs", Err: err}
	}

	// Translate the port's three-case return into *domain.DocumentIDFilter:
	//   - nil return: leave Filters.DocumentIDFilter untouched (no-op; provider declined).
	//   - empty slice: authoritative deny-all.
	//   - non-empty slice: authoritative allow-list.
	if allowedIDs != nil {
		parsed.SearchFilters.DocumentIDFilter = &domain.DocumentIDFilter{
			Apply: true,
			IDs:   allowedIDs,
		}
	}

	return parsed, nil
}

// Ensure DocumentIDFilterFactory implements StageFactory.
var _ pipelineport.StageFactory = (*DocumentIDFilterFactory)(nil)

// Ensure DocumentIDFilterStage implements Stage.
var _ pipelineport.Stage = (*DocumentIDFilterStage)(nil)
