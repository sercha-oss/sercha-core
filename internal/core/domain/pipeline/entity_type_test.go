package pipeline

import (
	"encoding/json"
	"testing"
)

func TestEntityType_ZeroValue(t *testing.T) {
	var et EntityType
	if et != "" {
		t.Errorf("zero value of EntityType = %q, want %q", et, "")
	}
}

func TestEntityType_StringConversion(t *testing.T) {
	et := EntityType("PERSON")
	if string(et) != "PERSON" {
		t.Errorf("string(EntityType) = %q, want %q", string(et), "PERSON")
	}
}

func TestEntityType_JSON_MarshalUnmarshal(t *testing.T) {
	type Container struct {
		Type EntityType `json:"type"`
	}

	original := Container{Type: EntityType("EMAIL")}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	// Unmarshal back
	var restored Container
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}

	if restored.Type != EntityType("EMAIL") {
		t.Errorf("after round-trip, Type = %q, want %q", restored.Type, "EMAIL")
	}
}

func TestEntityType_Comparison(t *testing.T) {
	et1 := EntityType("PERSON")
	et2 := EntityType("PERSON")
	et3 := EntityType("EMAIL")

	if et1 != et2 {
		t.Error("same EntityType values should be equal")
	}

	if et1 == et3 {
		t.Error("different EntityType values should not be equal")
	}
}
