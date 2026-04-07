package pgvector

import (
	"testing"
	"time"
)

// TestDefaultConfig validates that DefaultConfig returns expected default values
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		got      any
		want     any
		fieldMsg string
	}{
		{
			name:     "URL is empty by default",
			got:      cfg.URL,
			want:     "",
			fieldMsg: "URL",
		},
		{
			name:     "Dimensions is 1536 for OpenAI ada-002",
			got:      cfg.Dimensions,
			want:     1536,
			fieldMsg: "Dimensions",
		},
		{
			name:     "IndexType is hnsw",
			got:      cfg.IndexType,
			want:     "hnsw",
			fieldMsg: "IndexType",
		},
		{
			name:     "DistanceMetric is cosine",
			got:      cfg.DistanceMetric,
			want:     "cosine",
			fieldMsg: "DistanceMetric",
		},
		{
			name:     "MaxOpenConns is 10",
			got:      cfg.MaxOpenConns,
			want:     int32(10),
			fieldMsg: "MaxOpenConns",
		},
		{
			name:     "MinConns is 2",
			got:      cfg.MinConns,
			want:     int32(2),
			fieldMsg: "MinConns",
		},
		{
			name:     "MaxConnLifetime is 30 minutes",
			got:      cfg.MaxConnLifetime,
			want:     30 * time.Minute,
			fieldMsg: "MaxConnLifetime",
		},
		{
			name:     "MaxConnIdleTime is 5 minutes",
			got:      cfg.MaxConnIdleTime,
			want:     5 * time.Minute,
			fieldMsg: "MaxConnIdleTime",
		},
		{
			name:     "ConnTimeout is 10 seconds",
			got:      cfg.ConnTimeout,
			want:     10 * time.Second,
			fieldMsg: "ConnTimeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.fieldMsg, tt.got, tt.want)
			}
		})
	}
}

// TestDefaultConfig_NonZeroPoolSettings verifies pool settings are properly initialized
func TestDefaultConfig_NonZeroPoolSettings(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxOpenConns <= 0 {
		t.Error("MaxOpenConns should be positive")
	}
	if cfg.MinConns <= 0 {
		t.Error("MinConns should be positive")
	}
	if cfg.MaxConnLifetime <= 0 {
		t.Error("MaxConnLifetime should be positive")
	}
	if cfg.MaxConnIdleTime <= 0 {
		t.Error("MaxConnIdleTime should be positive")
	}
	if cfg.ConnTimeout <= 0 {
		t.Error("ConnTimeout should be positive")
	}
}

// TestDefaultConfig_MinConnsLessThanMaxConns verifies min conns is less than max conns
func TestDefaultConfig_MinConnsLessThanMaxConns(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MinConns > cfg.MaxOpenConns {
		t.Errorf("MinConns (%d) should be <= MaxOpenConns (%d)", cfg.MinConns, cfg.MaxOpenConns)
	}
}

// TestConfig_DistanceMetricValues tests the valid distance metric options
func TestConfig_DistanceMetricValues(t *testing.T) {
	validMetrics := []string{"cosine", "l2", "inner_product"}

	cfg := DefaultConfig()

	// Verify default is one of the valid metrics
	found := false
	for _, m := range validMetrics {
		if cfg.DistanceMetric == m {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Default DistanceMetric %q is not a valid metric", cfg.DistanceMetric)
	}
}

// TestConfig_IndexTypeValues tests the valid index type options
func TestConfig_IndexTypeValues(t *testing.T) {
	validTypes := []string{"hnsw", "ivfflat"}

	cfg := DefaultConfig()

	// Verify default is one of the valid types
	found := false
	for _, it := range validTypes {
		if cfg.IndexType == it {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Default IndexType %q is not a valid index type", cfg.IndexType)
	}
}
