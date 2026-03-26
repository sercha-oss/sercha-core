package pipeline

import (
	"github.com/custodia-labs/sercha-core/internal/core/domain/pipeline"
)

// StageRegistry holds available stage factories.
// Stages are registered at application startup.
type StageRegistry interface {
	// Register registers a stage factory.
	// Returns an error if a factory with the same ID is already registered.
	Register(factory StageFactory) error

	// Get retrieves a stage factory by stage ID.
	Get(stageID string) (StageFactory, bool)

	// List returns descriptors for all registered stages.
	List() []pipeline.StageDescriptor

	// ListByType returns descriptors for stages of a specific type.
	ListByType(stageType pipeline.StageType) []pipeline.StageDescriptor
}

// PipelineRegistry holds pipeline definitions.
type PipelineRegistry interface {
	// Register registers a pipeline definition.
	// Returns an error if a pipeline with the same ID is already registered.
	Register(def pipeline.PipelineDefinition) error

	// Get retrieves a pipeline definition by ID.
	Get(pipelineID string) (pipeline.PipelineDefinition, bool)

	// List returns all registered pipeline definitions.
	List() []pipeline.PipelineDefinition

	// ListByType returns pipeline definitions of a specific type.
	ListByType(pipelineType pipeline.PipelineType) []pipeline.PipelineDefinition

	// GetDefault returns the default pipeline for a given type.
	GetDefault(pipelineType pipeline.PipelineType) (pipeline.PipelineDefinition, bool)

	// SetDefault sets the default pipeline for a given type.
	SetDefault(pipelineType pipeline.PipelineType, pipelineID string) error
}

// CapabilityProvider provides a specific capability instance.
// Implementations wrap actual services (embedders, LLMs, stores).
type CapabilityProvider interface {
	// Type returns the capability type this provider offers.
	Type() pipeline.CapabilityType

	// ID returns the unique identifier for this provider instance.
	ID() string

	// Instance returns the actual service instance.
	// The concrete type depends on the capability type:
	// - CapabilityLLM: driven.LLMService
	// - CapabilityEmbedder: driven.EmbeddingService
	// - CapabilityVectorStore: driven.SearchEngine
	// - CapabilityDocStore: driven.DocumentStore
	// - CapabilityChunkStore: driven.DocumentStore (chunk operations)
	Instance() any

	// Available checks if the capability is currently available.
	Available() bool
}

// CapabilityRegistry holds available capability providers.
type CapabilityRegistry interface {
	// Register registers a capability provider.
	Register(provider CapabilityProvider) error

	// Unregister removes a capability provider.
	Unregister(capType pipeline.CapabilityType, id string) error

	// Get retrieves a specific capability provider.
	Get(capType pipeline.CapabilityType, id string) (CapabilityProvider, bool)

	// GetDefault retrieves the default provider for a capability type.
	GetDefault(capType pipeline.CapabilityType) (CapabilityProvider, bool)

	// SetDefault sets the default provider for a capability type.
	SetDefault(capType pipeline.CapabilityType, id string) error

	// List returns all providers of a capability type.
	List(capType pipeline.CapabilityType) []CapabilityProvider

	// ListAvailable returns all currently available providers of a type.
	ListAvailable(capType pipeline.CapabilityType) []CapabilityProvider

	// BuildCapabilitySet builds a CapabilitySet from registered providers.
	// Uses default providers for each required capability type.
	BuildCapabilitySet(required []pipeline.CapabilityRequirement) (*pipeline.CapabilitySet, error)
}

// ManifestStore persists and retrieves produces manifests.
type ManifestStore interface {
	// Save stores a produces manifest.
	Save(manifest *pipeline.ProducesManifest) error

	// Get retrieves the latest manifest for a connector.
	Get(connectorID string) (*pipeline.ProducesManifest, error)

	// GetByPipeline retrieves the latest manifest for a pipeline/connector pair.
	GetByPipeline(pipelineID, connectorID string) (*pipeline.ProducesManifest, error)

	// List returns all manifests.
	List() ([]*pipeline.ProducesManifest, error)

	// ListByConnector returns all manifests for a connector.
	ListByConnector(connectorID string) ([]*pipeline.ProducesManifest, error)
}

// EnablementStore persists search pipeline enablement configuration.
type EnablementStore interface {
	// Save saves or updates an enablement configuration.
	Save(enablement *pipeline.SearchPipelineEnablement) error

	// Get retrieves enablement for a pipeline.
	Get(pipelineID string) (*pipeline.SearchPipelineEnablement, bool)

	// ListEnabled returns all enabled search pipelines, sorted by priority.
	ListEnabled() []*pipeline.SearchPipelineEnablement

	// Delete removes an enablement configuration.
	Delete(pipelineID string) error
}
