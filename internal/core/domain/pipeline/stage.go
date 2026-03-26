package pipeline

// CapabilityRequirement specifies a capability dependency for a stage.
type CapabilityRequirement struct {
	Type     CapabilityType `json:"type"`
	Mode     CapabilityMode `json:"mode"`
	MinCount int            `json:"min_count,omitempty"` // Minimum instances needed (default 1)
	Config   any            `json:"config,omitempty"`    // Capability-specific config
}

// StageDescriptor contains static metadata about a stage implementation.
type StageDescriptor struct {
	ID           string                  `json:"id"`           // Unique identifier (e.g., "semantic-chunker")
	Name         string                  `json:"name"`         // Human-readable name
	Type         StageType               `json:"type"`         // Category
	InputShape   ShapeName               `json:"input_shape"`  // Expected input shape
	OutputShape  ShapeName               `json:"output_shape"` // Produced output shape
	Cardinality  Cardinality             `json:"cardinality"`  // I/O relationship
	Capabilities []CapabilityRequirement `json:"capabilities"` // Required/optional capabilities
	Version      string                  `json:"version"`      // Semantic version
}

// StageConfig contains runtime configuration for a stage instance.
type StageConfig struct {
	StageID    string         `json:"stage_id"`   // References StageDescriptor.ID
	Parameters map[string]any `json:"parameters"` // Stage-specific parameters
	Enabled    bool           `json:"enabled"`    // Can be toggled off
}

// RequiresCapability checks if the stage requires a specific capability type.
func (d *StageDescriptor) RequiresCapability(capType CapabilityType) bool {
	for _, req := range d.Capabilities {
		if req.Type == capType && req.Mode == CapabilityRequired {
			return true
		}
	}
	return false
}

// GetRequiredCapabilities returns all required capability types.
func (d *StageDescriptor) GetRequiredCapabilities() []CapabilityType {
	var required []CapabilityType
	for _, req := range d.Capabilities {
		if req.Mode == CapabilityRequired {
			required = append(required, req.Type)
		}
	}
	return required
}
