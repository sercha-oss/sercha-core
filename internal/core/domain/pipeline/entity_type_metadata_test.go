package pipeline

import (
	"encoding/json"
	"testing"
)

func TestEntityTypeMetadata_ZeroValue(t *testing.T) {
	var metadata EntityTypeMetadata

	if metadata.ID != "" {
		t.Errorf("zero value ID = %q, want %q", metadata.ID, "")
	}
	if metadata.DisplayName != "" {
		t.Errorf("zero value DisplayName = %q, want %q", metadata.DisplayName, "")
	}
	if metadata.Description != "" {
		t.Errorf("zero value Description = %q, want %q", metadata.Description, "")
	}
	if metadata.Example != "" {
		t.Errorf("zero value Example = %q, want %q", metadata.Example, "")
	}
	if metadata.Group != "" {
		t.Errorf("zero value Group = %q, want %q", metadata.Group, "")
	}
	if metadata.Source != "" {
		t.Errorf("zero value Source = %q, want %q", metadata.Source, "")
	}
	if metadata.OwningDetector != "" {
		t.Errorf("zero value OwningDetector = %q, want %q", metadata.OwningDetector, "")
	}
}

func TestEntityTypeMetadata_JSON_MarshalUnmarshal(t *testing.T) {
	original := EntityTypeMetadata{
		ID:             EntityType("PERSON"),
		DisplayName:    "Person",
		Description:    "A human individual",
		Example:        "John Doe",
		Group:          "PII",
		Source:         "system",
		OwningDetector: "nlp-detector",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	// Verify JSON structure
	var jsonObj map[string]interface{}
	err = json.Unmarshal(data, &jsonObj)
	if err != nil {
		t.Fatalf("json.Unmarshal to map error = %v", err)
	}

	// Verify all fields are present (no omitempty)
	expectedFields := []string{"id", "display_name", "description", "example", "group", "source", "owning_detector"}
	for _, field := range expectedFields {
		if _, ok := jsonObj[field]; !ok {
			t.Errorf("JSON missing field: %q", field)
		}
	}

	// Unmarshal back to struct
	var restored EntityTypeMetadata
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal to struct error = %v", err)
	}

	if restored.ID != EntityType("PERSON") {
		t.Errorf("after round-trip, ID = %q, want %q", restored.ID, "PERSON")
	}
	if restored.DisplayName != "Person" {
		t.Errorf("after round-trip, DisplayName = %q, want %q", restored.DisplayName, "Person")
	}
	if restored.Description != "A human individual" {
		t.Errorf("after round-trip, Description = %q, want %q", restored.Description, "A human individual")
	}
	if restored.Example != "John Doe" {
		t.Errorf("after round-trip, Example = %q, want %q", restored.Example, "John Doe")
	}
	if restored.Group != "PII" {
		t.Errorf("after round-trip, Group = %q, want %q", restored.Group, "PII")
	}
	if restored.Source != "system" {
		t.Errorf("after round-trip, Source = %q, want %q", restored.Source, "system")
	}
	if restored.OwningDetector != "nlp-detector" {
		t.Errorf("after round-trip, OwningDetector = %q, want %q", restored.OwningDetector, "nlp-detector")
	}
}

func TestEntityTypeMetadata_JSON_PreservesEmptyFields(t *testing.T) {
	// Test that empty fields are preserved in JSON (not omitted)
	metadata := EntityTypeMetadata{
		ID:             EntityType("EMAIL"),
		DisplayName:    "Email Address",
		Description:    "An email address",
		Example:        "user@example.com",
		Group:          "", // Empty group
		Source:         "system",
		OwningDetector: "", // No owner
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	// Check that empty fields are in the JSON
	jsonStr := string(data)
	if !contains(jsonStr, "group") {
		t.Error("group field should be present in JSON even when empty")
	}
	if !contains(jsonStr, "owning_detector") {
		t.Error("owning_detector field should be present in JSON even when empty")
	}

	// Unmarshal and verify empty values are preserved
	var restored EntityTypeMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.Group != "" {
		t.Errorf("restored Group = %q, want %q", restored.Group, "")
	}
	if restored.OwningDetector != "" {
		t.Errorf("restored OwningDetector = %q, want %q", restored.OwningDetector, "")
	}
}

func TestEntityTypeMetadata_JSON_AllFields(t *testing.T) {
	tests := []struct {
		name     string
		metadata EntityTypeMetadata
	}{
		{
			name: "system builtin",
			metadata: EntityTypeMetadata{
				ID:             EntityType("PERSON"),
				DisplayName:    "Person",
				Description:    "A human individual",
				Example:        "John Doe",
				Group:          "PII",
				Source:         "system",
				OwningDetector: "",
			},
		},
		{
			name: "admin added with owner",
			metadata: EntityTypeMetadata{
				ID:             EntityType("CUSTOM_CODE"),
				DisplayName:    "Custom Code",
				Description:    "A custom code pattern",
				Example:        "ABC123",
				Group:          "Custom",
				Source:         "admin",
				OwningDetector: "custom-detector",
			},
		},
		{
			name: "minimal",
			metadata: EntityTypeMetadata{
				ID:             EntityType("MINIMAL"),
				DisplayName:    "Minimal",
				Description:    "",
				Example:        "",
				Group:          "",
				Source:         "",
				OwningDetector: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.metadata)
			var restored EntityTypeMetadata
			err := json.Unmarshal(data, &restored)
			if err != nil {
				t.Fatalf("json round-trip error: %v", err)
			}

			if restored.ID != tt.metadata.ID {
				t.Errorf("ID mismatch: %q != %q", restored.ID, tt.metadata.ID)
			}
			if restored.DisplayName != tt.metadata.DisplayName {
				t.Errorf("DisplayName mismatch: %q != %q", restored.DisplayName, tt.metadata.DisplayName)
			}
			if restored.Description != tt.metadata.Description {
				t.Errorf("Description mismatch: %q != %q", restored.Description, tt.metadata.Description)
			}
			if restored.Example != tt.metadata.Example {
				t.Errorf("Example mismatch: %q != %q", restored.Example, tt.metadata.Example)
			}
			if restored.Group != tt.metadata.Group {
				t.Errorf("Group mismatch: %q != %q", restored.Group, tt.metadata.Group)
			}
			if restored.Source != tt.metadata.Source {
				t.Errorf("Source mismatch: %q != %q", restored.Source, tt.metadata.Source)
			}
			if restored.OwningDetector != tt.metadata.OwningDetector {
				t.Errorf("OwningDetector mismatch: %q != %q", restored.OwningDetector, tt.metadata.OwningDetector)
			}
		})
	}
}

func TestEntityTypeMetadata_Comparison(t *testing.T) {
	m1 := EntityTypeMetadata{
		ID:             EntityType("TYPE1"),
		DisplayName:    "Type 1",
		Description:    "Description 1",
		Example:        "Example 1",
		Group:          "Group 1",
		Source:         "system",
		OwningDetector: "detector-1",
	}

	m2 := EntityTypeMetadata{
		ID:             EntityType("TYPE1"),
		DisplayName:    "Type 1",
		Description:    "Description 1",
		Example:        "Example 1",
		Group:          "Group 1",
		Source:         "system",
		OwningDetector: "detector-1",
	}

	m3 := EntityTypeMetadata{
		ID:             EntityType("TYPE2"),
		DisplayName:    "Type 2",
		Description:    "Description 2",
		Example:        "Example 2",
		Group:          "Group 2",
		Source:         "admin",
		OwningDetector: "detector-2",
	}

	if m1 != m2 {
		t.Error("identical metadata should be equal")
	}

	if m1 == m3 {
		t.Error("different metadata should not be equal")
	}
}

func TestEntityTypeMetadata_OwningDetectorField(t *testing.T) {
	// Test that OwningDetector field can be set and retrieved
	metadata := EntityTypeMetadata{
		ID:             EntityType("CLAIMED"),
		DisplayName:    "Claimed Type",
		Description:    "A claimed entity type",
		Example:        "example",
		Group:          "",
		Source:         "system",
		OwningDetector: "my-detector-v2",
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var restored EntityTypeMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.OwningDetector != "my-detector-v2" {
		t.Errorf("OwningDetector = %q, want %q", restored.OwningDetector, "my-detector-v2")
	}
}

func TestEntityTypeMetadata_EmptyOwningDetector_Meaningful(t *testing.T) {
	// Empty OwningDetector is a meaningful state (unclaimed)
	m1 := EntityTypeMetadata{
		ID:             EntityType("UNCLAIMED"),
		DisplayName:    "Unclaimed",
		Description:    "Not owned by any detector",
		Example:        "example",
		Group:          "",
		Source:         "system",
		OwningDetector: "", // unclaimed
	}

	m2 := EntityTypeMetadata{
		ID:             EntityType("UNCLAIMED"),
		DisplayName:    "Unclaimed",
		Description:    "Not owned by any detector",
		Example:        "example",
		Group:          "",
		Source:         "system",
		OwningDetector: "detector-x", // claimed
	}

	if m1 == m2 {
		t.Error("unclaimed and claimed should not be equal")
	}

	data1, _ := json.Marshal(m1)
	data2, _ := json.Marshal(m2)

	if string(data1) == string(data2) {
		t.Error("JSON should differ for unclaimed vs claimed")
	}
}
