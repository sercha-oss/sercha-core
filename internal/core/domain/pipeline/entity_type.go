package pipeline

// EntityType is an open-string identifier for a category of named entity.
//
// No constants are declared in this package. Consumers register entity types
// at runtime via the EntityTypeRegistry port. This mirrors the open-string
// pattern used by ProviderType and CapabilityType elsewhere in the domain.
type EntityType string
