package pipeline

import (
	"encoding/json"
	"testing"
)

func TestEntitySpan_ZeroValue(t *testing.T) {
	var span EntitySpan

	if span.Type != "" {
		t.Errorf("zero value Type = %q, want %q", span.Type, "")
	}
	if span.Value != "" {
		t.Errorf("zero value Value = %q, want %q", span.Value, "")
	}
	if span.Start != 0 {
		t.Errorf("zero value Start = %d, want 0", span.Start)
	}
	if span.End != 0 {
		t.Errorf("zero value End = %d, want 0", span.End)
	}
	if span.Confidence != 0.0 {
		t.Errorf("zero value Confidence = %f, want 0.0", span.Confidence)
	}
	if span.Detector != "" {
		t.Errorf("zero value Detector = %q, want %q", span.Detector, "")
	}
}

func TestEntitySpan_JSON_MarshalUnmarshal(t *testing.T) {
	original := EntitySpan{
		Type:       EntityType("PERSON"),
		Value:      "John Doe",
		Start:      0,
		End:        8,
		Confidence: 0.95,
		Detector:   "test-detector",
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

	if jsonObj["type"] != "PERSON" {
		t.Errorf("JSON type = %v, want PERSON", jsonObj["type"])
	}
	if jsonObj["value"] != "John Doe" {
		t.Errorf("JSON value = %v, want John Doe", jsonObj["value"])
	}
	if int(jsonObj["start"].(float64)) != 0 {
		t.Errorf("JSON start = %v, want 0", jsonObj["start"])
	}
	if int(jsonObj["end"].(float64)) != 8 {
		t.Errorf("JSON end = %v, want 8", jsonObj["end"])
	}

	// Unmarshal back to struct
	var restored EntitySpan
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal to struct error = %v", err)
	}

	if restored.Type != EntityType("PERSON") {
		t.Errorf("after round-trip, Type = %q, want %q", restored.Type, "PERSON")
	}
	if restored.Value != "John Doe" {
		t.Errorf("after round-trip, Value = %q, want %q", restored.Value, "John Doe")
	}
	if restored.Start != 0 {
		t.Errorf("after round-trip, Start = %d, want 0", restored.Start)
	}
	if restored.End != 8 {
		t.Errorf("after round-trip, End = %d, want 8", restored.End)
	}
	if restored.Confidence != 0.95 {
		t.Errorf("after round-trip, Confidence = %f, want 0.95", restored.Confidence)
	}
	if restored.Detector != "test-detector" {
		t.Errorf("after round-trip, Detector = %q, want %q", restored.Detector, "test-detector")
	}
}

func TestEntitySpan_JSON_PreservesZeroValues(t *testing.T) {
	// Zero confidence should be preserved in JSON (not omitted)
	span := EntitySpan{
		Type:       EntityType("RULE_BASED"),
		Value:      "pattern",
		Start:      5,
		End:        12,
		Confidence: 0.0, // Zero confidence
		Detector:   "regex-detector",
	}

	data, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("json.Marshal error = %v", err)
	}

	// Check that confidence is in the JSON
	if !contains(string(data), "confidence") {
		t.Error("confidence field should be present in JSON even when 0.0")
	}

	// Unmarshal and verify
	var restored EntitySpan
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.Confidence != 0.0 {
		t.Errorf("restored Confidence = %f, want 0.0", restored.Confidence)
	}
}

func TestEntitySpan_ConfidenceRange(t *testing.T) {
	tests := []float64{
		0.0,  // minimum
		0.5,  // midpoint
		1.0,  // maximum
		0.99, // high confidence
		0.01, // low confidence
	}

	for _, conf := range tests {
		span := EntitySpan{
			Type:       EntityType("TEST"),
			Value:      "value",
			Start:      0,
			End:        5,
			Confidence: conf,
			Detector:   "test",
		}

		data, err := json.Marshal(span)
		if err != nil {
			t.Fatalf("Marshal failed for confidence %f: %v", conf, err)
		}
		var restored EntitySpan
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatalf("Unmarshal failed for confidence %f: %v", conf, err)
		}

		if restored.Confidence != conf {
			t.Errorf("Confidence %f not preserved in JSON round-trip", conf)
		}
	}
}

func TestEntitySpan_ByteOffsets(t *testing.T) {
	// Test that byte offsets work correctly
	// "Hello World" → "World" starts at byte 6
	span := EntitySpan{
		Type:       EntityType("WORD"),
		Value:      "World",
		Start:      6,
		End:        11,
		Confidence: 1.0,
		Detector:   "test",
	}

	content := "Hello World"
	extracted := content[span.Start:span.End]
	if extracted != "World" {
		t.Errorf("content[%d:%d] = %q, want %q", span.Start, span.End, extracted, "World")
	}

	// Verify round-trip
	data, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var restored EntitySpan
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.Start != 6 || restored.End != 11 {
		t.Errorf("offsets not preserved: Start=%d, End=%d", restored.Start, restored.End)
	}

	extracted2 := content[restored.Start:restored.End]
	if extracted2 != "World" {
		t.Errorf("after round-trip, content[%d:%d] = %q, want %q", restored.Start, restored.End, extracted2, "World")
	}
}

func TestEntitySpan_UTF8Offsets(t *testing.T) {
	// Byte offsets should work with UTF-8
	// "Café" has 'é' as 2 bytes; 'Café' is 5 bytes total
	content := "Café"
	span := EntitySpan{
		Type:       EntityType("WORD"),
		Value:      "Café",
		Start:      0,
		End:        5, // 4 characters, 5 bytes
		Confidence: 1.0,
		Detector:   "test",
	}

	data, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var restored EntitySpan
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	extracted := content[restored.Start:restored.End]
	if extracted != "Café" {
		t.Errorf("UTF-8 content[%d:%d] = %q, want %q", restored.Start, restored.End, extracted, "Café")
	}
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
