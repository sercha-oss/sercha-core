package domain

import (
	"testing"
	"time"
)

// Test CapabilityType constants
func TestCapabilityTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		capType  CapabilityType
		expected string
	}{
		{"text indexing", CapabilityTextIndexing, "text_indexing"},
		{"embedding indexing", CapabilityEmbeddingIndexing, "embedding_indexing"},
		{"bm25 search", CapabilityBM25Search, "bm25_search"},
		{"vector search", CapabilityVectorSearch, "vector_search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.capType) != tt.expected {
				t.Errorf("got %q, want %q", tt.capType, tt.expected)
			}
		})
	}
}

// Test PipelinePhase constants
func TestPipelinePhaseConstants(t *testing.T) {
	tests := []struct {
		name     string
		phase    PipelinePhase
		expected string
	}{
		{"indexing phase", PipelinePhaseIndexing, "indexing"},
		{"search phase", PipelinePhaseSearch, "search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.phase) != tt.expected {
				t.Errorf("got %q, want %q", tt.phase, tt.expected)
			}
		})
	}
}

// Test Capability.IsActive
func TestCapability_IsActive(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		enabled   bool
		want      bool
	}{
		{"available and enabled", true, true, true},
		{"available but disabled", true, false, false},
		{"unavailable but enabled", false, true, false},
		{"unavailable and disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capability{
				Available: tt.available,
				Enabled:   tt.enabled,
			}
			if got := c.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test Capability.CanBeEnabled
func TestCapability_CanBeEnabled(t *testing.T) {
	tests := []struct {
		name      string
		available bool
		want      bool
	}{
		{"available capability", true, true},
		{"unavailable capability", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capability{
				Available: tt.available,
			}
			if got := c.CanBeEnabled(); got != tt.want {
				t.Errorf("CanBeEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test Capability.IsIndexingCapability
func TestCapability_IsIndexingCapability(t *testing.T) {
	tests := []struct {
		name  string
		phase PipelinePhase
		want  bool
	}{
		{"indexing phase", PipelinePhaseIndexing, true},
		{"search phase", PipelinePhaseSearch, false},
		{"empty phase", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capability{
				Phase: tt.phase,
			}
			if got := c.IsIndexingCapability(); got != tt.want {
				t.Errorf("IsIndexingCapability() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test Capability.IsSearchCapability
func TestCapability_IsSearchCapability(t *testing.T) {
	tests := []struct {
		name  string
		phase PipelinePhase
		want  bool
	}{
		{"search phase", PipelinePhaseSearch, true},
		{"indexing phase", PipelinePhaseIndexing, false},
		{"empty phase", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capability{
				Phase: tt.phase,
			}
			if got := c.IsSearchCapability(); got != tt.want {
				t.Errorf("IsSearchCapability() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test Capability.HasBackend
func TestCapability_HasBackend(t *testing.T) {
	tests := []struct {
		name      string
		backendID string
		want      bool
	}{
		{"with backend", "opensearch", true},
		{"with pgvector backend", "pgvector", true},
		{"empty backend", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Capability{
				BackendID: tt.backendID,
			}
			if got := c.HasBackend(); got != tt.want {
				t.Errorf("HasBackend() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test NewTextIndexingCapability
func TestNewTextIndexingCapability(t *testing.T) {
	tests := []struct {
		name      string
		backendID string
		available bool
	}{
		{"available with opensearch", "opensearch", true},
		{"unavailable", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := NewTextIndexingCapability(tt.backendID, tt.available)

			if cap.ID != "text_indexing" {
				t.Errorf("ID = %q, want %q", cap.ID, "text_indexing")
			}
			if cap.Type != CapabilityTextIndexing {
				t.Errorf("Type = %v, want %v", cap.Type, CapabilityTextIndexing)
			}
			if cap.Phase != PipelinePhaseIndexing {
				t.Errorf("Phase = %v, want %v", cap.Phase, PipelinePhaseIndexing)
			}
			if cap.BackendID != tt.backendID {
				t.Errorf("BackendID = %q, want %q", cap.BackendID, tt.backendID)
			}
			if cap.Available != tt.available {
				t.Errorf("Available = %v, want %v", cap.Available, tt.available)
			}
			if !cap.Enabled {
				t.Errorf("Enabled = %v, want true", cap.Enabled)
			}
			if len(cap.Grants) != 1 || cap.Grants[0] != CapabilityBM25Search {
				t.Errorf("Grants = %v, want [%v]", cap.Grants, CapabilityBM25Search)
			}
		})
	}
}

// Test NewEmbeddingIndexingCapability
func TestNewEmbeddingIndexingCapability(t *testing.T) {
	tests := []struct {
		name      string
		backendID string
		available bool
	}{
		{"available with pgvector", "pgvector", true},
		{"unavailable", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := NewEmbeddingIndexingCapability(tt.backendID, tt.available)

			if cap.ID != "embedding_indexing" {
				t.Errorf("ID = %q, want %q", cap.ID, "embedding_indexing")
			}
			if cap.Type != CapabilityEmbeddingIndexing {
				t.Errorf("Type = %v, want %v", cap.Type, CapabilityEmbeddingIndexing)
			}
			if cap.Phase != PipelinePhaseIndexing {
				t.Errorf("Phase = %v, want %v", cap.Phase, PipelinePhaseIndexing)
			}
			if cap.BackendID != tt.backendID {
				t.Errorf("BackendID = %q, want %q", cap.BackendID, tt.backendID)
			}
			if cap.Available != tt.available {
				t.Errorf("Available = %v, want %v", cap.Available, tt.available)
			}
			if cap.Enabled {
				t.Errorf("Enabled = %v, want false", cap.Enabled)
			}
			if len(cap.Grants) != 1 || cap.Grants[0] != CapabilityVectorSearch {
				t.Errorf("Grants = %v, want [%v]", cap.Grants, CapabilityVectorSearch)
			}
		})
	}
}

// Test NewBM25SearchCapability
func TestNewBM25SearchCapability(t *testing.T) {
	tests := []struct {
		name      string
		backendID string
		available bool
	}{
		{"available with opensearch", "opensearch", true},
		{"unavailable", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := NewBM25SearchCapability(tt.backendID, tt.available)

			if cap.ID != "bm25_search" {
				t.Errorf("ID = %q, want %q", cap.ID, "bm25_search")
			}
			if cap.Type != CapabilityBM25Search {
				t.Errorf("Type = %v, want %v", cap.Type, CapabilityBM25Search)
			}
			if cap.Phase != PipelinePhaseSearch {
				t.Errorf("Phase = %v, want %v", cap.Phase, PipelinePhaseSearch)
			}
			if cap.BackendID != tt.backendID {
				t.Errorf("BackendID = %q, want %q", cap.BackendID, tt.backendID)
			}
			if cap.Available != tt.available {
				t.Errorf("Available = %v, want %v", cap.Available, tt.available)
			}
			if !cap.Enabled {
				t.Errorf("Enabled = %v, want true", cap.Enabled)
			}
			if len(cap.DependsOn) != 1 || cap.DependsOn[0] != CapabilityTextIndexing {
				t.Errorf("DependsOn = %v, want [%v]", cap.DependsOn, CapabilityTextIndexing)
			}
		})
	}
}

// Test NewVectorSearchCapability
func TestNewVectorSearchCapability(t *testing.T) {
	tests := []struct {
		name      string
		backendID string
		available bool
	}{
		{"available with pgvector", "pgvector", true},
		{"unavailable", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cap := NewVectorSearchCapability(tt.backendID, tt.available)

			if cap.ID != "vector_search" {
				t.Errorf("ID = %q, want %q", cap.ID, "vector_search")
			}
			if cap.Type != CapabilityVectorSearch {
				t.Errorf("Type = %v, want %v", cap.Type, CapabilityVectorSearch)
			}
			if cap.Phase != PipelinePhaseSearch {
				t.Errorf("Phase = %v, want %v", cap.Phase, PipelinePhaseSearch)
			}
			if cap.BackendID != tt.backendID {
				t.Errorf("BackendID = %q, want %q", cap.BackendID, tt.backendID)
			}
			if cap.Available != tt.available {
				t.Errorf("Available = %v, want %v", cap.Available, tt.available)
			}
			if !cap.Enabled {
				t.Errorf("Enabled = %v, want true", cap.Enabled)
			}
			if len(cap.DependsOn) != 1 || cap.DependsOn[0] != CapabilityEmbeddingIndexing {
				t.Errorf("DependsOn = %v, want [%v]", cap.DependsOn, CapabilityEmbeddingIndexing)
			}
		})
	}
}

// Test DefaultCapabilityPreferences
func TestDefaultCapabilityPreferences(t *testing.T) {
	teamID := "team-123"
	prefs := DefaultCapabilityPreferences(teamID)

	if prefs.TeamID != teamID {
		t.Errorf("TeamID = %q, want %q", prefs.TeamID, teamID)
	}
	if !prefs.TextIndexingEnabled {
		t.Error("TextIndexingEnabled = false, want true")
	}
	if prefs.EmbeddingIndexingEnabled {
		t.Error("EmbeddingIndexingEnabled = true, want false")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("BM25SearchEnabled = false, want true")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("VectorSearchEnabled = false, want true")
	}
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt is zero, want non-zero time")
	}
}

// Test CapabilityPreferences.HasTextIndexing
func TestCapabilityPreferences_HasTextIndexing(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs := &CapabilityPreferences{
				TextIndexingEnabled: tt.enabled,
			}
			if got := prefs.HasTextIndexing(); got != tt.want {
				t.Errorf("HasTextIndexing() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test CapabilityPreferences.HasEmbeddingIndexing
func TestCapabilityPreferences_HasEmbeddingIndexing(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{"enabled", true, true},
		{"disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs := &CapabilityPreferences{
				EmbeddingIndexingEnabled: tt.enabled,
			}
			if got := prefs.HasEmbeddingIndexing(); got != tt.want {
				t.Errorf("HasEmbeddingIndexing() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test CapabilityPreferences.CanUseBM25Search
func TestCapabilityPreferences_CanUseBM25Search(t *testing.T) {
	tests := []struct {
		name          string
		textIndexing  bool
		bm25Search    bool
		want          bool
	}{
		{"both enabled", true, true, true},
		{"text disabled", false, true, false},
		{"bm25 disabled", true, false, false},
		{"both disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs := &CapabilityPreferences{
				TextIndexingEnabled: tt.textIndexing,
				BM25SearchEnabled:   tt.bm25Search,
			}
			if got := prefs.CanUseBM25Search(); got != tt.want {
				t.Errorf("CanUseBM25Search() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test CapabilityPreferences.CanUseVectorSearch
func TestCapabilityPreferences_CanUseVectorSearch(t *testing.T) {
	tests := []struct {
		name             string
		embeddingIndexing bool
		vectorSearch     bool
		want             bool
	}{
		{"both enabled", true, true, true},
		{"embedding disabled", false, true, false},
		{"vector disabled", true, false, false},
		{"both disabled", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefs := &CapabilityPreferences{
				EmbeddingIndexingEnabled: tt.embeddingIndexing,
				VectorSearchEnabled:      tt.vectorSearch,
			}
			if got := prefs.CanUseVectorSearch(); got != tt.want {
				t.Errorf("CanUseVectorSearch() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test CapabilityPreferences.EnableTextIndexing
func TestCapabilityPreferences_EnableTextIndexing(t *testing.T) {
	prefs := &CapabilityPreferences{
		TextIndexingEnabled: false,
		BM25SearchEnabled:   false,
		UpdatedAt:           time.Time{},
	}

	prefs.EnableTextIndexing()

	if !prefs.TextIndexingEnabled {
		t.Error("TextIndexingEnabled = false, want true")
	}
	if !prefs.BM25SearchEnabled {
		t.Error("BM25SearchEnabled = false, want true")
	}
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not updated")
	}
}

// Test CapabilityPreferences.DisableTextIndexing
func TestCapabilityPreferences_DisableTextIndexing(t *testing.T) {
	prefs := &CapabilityPreferences{
		TextIndexingEnabled: true,
		BM25SearchEnabled:   true,
		UpdatedAt:           time.Time{},
	}

	prefs.DisableTextIndexing()

	if prefs.TextIndexingEnabled {
		t.Error("TextIndexingEnabled = true, want false")
	}
	if prefs.BM25SearchEnabled {
		t.Error("BM25SearchEnabled = true, want false")
	}
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not updated")
	}
}

// Test CapabilityPreferences.EnableEmbeddingIndexing
func TestCapabilityPreferences_EnableEmbeddingIndexing(t *testing.T) {
	prefs := &CapabilityPreferences{
		EmbeddingIndexingEnabled: false,
		VectorSearchEnabled:      false,
		UpdatedAt:                time.Time{},
	}

	prefs.EnableEmbeddingIndexing()

	if !prefs.EmbeddingIndexingEnabled {
		t.Error("EmbeddingIndexingEnabled = false, want true")
	}
	if !prefs.VectorSearchEnabled {
		t.Error("VectorSearchEnabled = false, want true")
	}
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not updated")
	}
}

// Test CapabilityPreferences.DisableEmbeddingIndexing
func TestCapabilityPreferences_DisableEmbeddingIndexing(t *testing.T) {
	prefs := &CapabilityPreferences{
		EmbeddingIndexingEnabled: true,
		VectorSearchEnabled:      true,
		UpdatedAt:                time.Time{},
	}

	prefs.DisableEmbeddingIndexing()

	if prefs.EmbeddingIndexingEnabled {
		t.Error("EmbeddingIndexingEnabled = true, want false")
	}
	if prefs.VectorSearchEnabled {
		t.Error("VectorSearchEnabled = true, want false")
	}
	if prefs.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not updated")
	}
}
