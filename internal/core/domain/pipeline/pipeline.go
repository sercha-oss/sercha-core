package pipeline

// PipelineDefinition is the complete specification of a pipeline.
type PipelineDefinition struct {
	ID          string        `json:"id"`          // Unique identifier (e.g., "default-indexing")
	Name        string        `json:"name"`        // Human-readable name
	Type        PipelineType  `json:"type"`        // "indexing" or "search"
	Stages      []StageConfig `json:"stages"`      // Ordered list of stages
	Version     string        `json:"version"`     // Semantic version
	Description string        `json:"description"` // What this pipeline does
}
