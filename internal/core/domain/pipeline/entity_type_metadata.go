// EntityTypeMetadata describes a single registered entity type in the taxonomy:
// its identifier, human-readable display fields, and provenance information.
// The taxonomy is managed at runtime through the EntityTypeRegistry port;
// this file owns only the data shape.

package pipeline

// EntityTypeMetadata describes a single registered entity type in the taxonomy.
type EntityTypeMetadata struct {
	// ID is the open-string identifier for this entity type.
	ID EntityType `json:"id"`

	// DisplayName is the human-readable label used for UI rendering.
	DisplayName string `json:"display_name"`

	// Description is explanatory text for the entity type. May be used in
	// detector prompts as part of few-shot or instruction context.
	Description string `json:"description"`

	// Example is a representative value for this entity type. Used in UI
	// and as a few-shot example in detector prompts.
	Example string `json:"example"`

	// Group is an optional grouping label for UI or admin segmentation
	// (e.g. "PII", "Medical", "Financial"). Empty means ungrouped.
	Group string `json:"group"`

	// Source identifies provenance: "system" for built-in types, "admin"
	// for types added at runtime by an administrator.
	Source string `json:"source"`

	// OwningDetector is the identity of the detector that has claimed this
	// category. Empty by default; set via SetOwningDetector on the registry
	// when a detector claims the category. Used for partition validation when
	// multiple detectors are registered.
	OwningDetector string `json:"owning_detector"`
}
