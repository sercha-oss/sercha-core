package pgvector

import (
	"testing"
)

// TestMaxEmbeddedVersion verifies the pgvector adapter reports its single
// registered migration. The DB-touching paths (Up/Down/Status/Version/
// EnsureClean) are exercised via the integration tests under TEST_DATABASE_URL
// in a follow-up environment; this is the always-runnable smoke test.
func TestMaxEmbeddedVersion(t *testing.T) {
	v, err := MaxEmbeddedVersion()
	if err != nil {
		t.Fatalf("MaxEmbeddedVersion returned error: %v", err)
	}
	if v != 1 {
		t.Errorf("expected MaxEmbeddedVersion = 1, got %d", v)
	}
}
