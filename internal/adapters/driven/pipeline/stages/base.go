package stages

import (
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/indexing"
	"github.com/sercha-oss/sercha-core/internal/adapters/driven/pipeline/stages/search"
	"github.com/sercha-oss/sercha-core/internal/core/domain/pipeline"
	pipelineport "github.com/sercha-oss/sercha-core/internal/core/ports/driven/pipeline"
)

// BaseStageFactory provides common functionality for stage factories.
type BaseStageFactory struct {
	descriptor pipeline.StageDescriptor
}

// NewBaseStageFactory creates a new base factory with the given descriptor.
func NewBaseStageFactory(descriptor pipeline.StageDescriptor) BaseStageFactory {
	return BaseStageFactory{descriptor: descriptor}
}

// StageID returns the unique identifier for stages this factory creates.
func (f *BaseStageFactory) StageID() string {
	return f.descriptor.ID
}

// Descriptor returns the stage descriptor.
func (f *BaseStageFactory) Descriptor() pipeline.StageDescriptor {
	return f.descriptor
}

// Validate provides default validation (checks stage ID matches).
func (f *BaseStageFactory) Validate(config pipeline.StageConfig) error {
	return nil
}

// GetCapability retrieves a typed capability from the set.
func GetCapability[T any](capabilities *pipeline.CapabilitySet, capType pipeline.CapabilityType) (T, bool) {
	var zero T
	if capabilities == nil {
		return zero, false
	}

	inst, ok := capabilities.Get(capType)
	if !ok {
		return zero, false
	}

	typed, ok := inst.Instance.(T)
	return typed, ok
}

// MustGetCapability retrieves a typed capability or panics.
func MustGetCapability[T any](capabilities *pipeline.CapabilitySet, capType pipeline.CapabilityType) T {
	val, ok := GetCapability[T](capabilities, capType)
	if !ok {
		panic("required capability not found: " + string(capType))
	}
	return val
}

// RegisterAll registers all built-in stage factories with the registry.
func RegisterAll(registry pipelineport.StageRegistry) error {
	factories := []pipelineport.StageFactory{
		// Indexing stages
		indexing.NewChunkerFactory(),
		indexing.NewEmbedderFactory(),
		indexing.NewDocLoaderFactory(),
		indexing.NewVectorLoaderFactory(),

		// Search stages
		search.NewQueryParserFactory(),
		search.NewMultiRetrieverFactory(),
		search.NewRankerFactory(),
		search.NewPresenterFactory(),
	}

	for _, factory := range factories {
		if err := registry.Register(factory); err != nil {
			return err
		}
	}

	return nil
}
