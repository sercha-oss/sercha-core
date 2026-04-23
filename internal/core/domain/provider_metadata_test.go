package domain

import "testing"

func TestAllProviderMetadata_CoversEveryKnownProvider(t *testing.T) {
	allProviders := []ProviderType{
		ProviderTypeGitHub,
		ProviderTypeLocalFS,
		ProviderTypeNotion,
		ProviderTypeOneDrive,
	}
	metadata := AllProviderMetadata()

	if len(metadata) != len(allProviders) {
		t.Fatalf("AllProviderMetadata returned %d entries, expected %d (one per ProviderType const)",
			len(metadata), len(allProviders))
	}

	seen := map[ProviderType]bool{}
	for _, m := range metadata {
		if seen[m.Type] {
			t.Errorf("duplicate metadata entry for %s", m.Type)
		}
		seen[m.Type] = true
		if m.DisplayName == "" {
			t.Errorf("provider %s has empty DisplayName", m.Type)
		}
		if m.IconID == "" {
			t.Errorf("provider %s has empty IconID", m.Type)
		}
	}
	for _, p := range allProviders {
		if !seen[p] {
			t.Errorf("provider %s has no metadata entry", p)
		}
	}
}

func TestMetadataFor(t *testing.T) {
	m, ok := MetadataFor(ProviderTypeGitHub)
	if !ok {
		t.Fatal("MetadataFor(GitHub) returned ok=false")
	}
	if m.DisplayName != "GitHub" {
		t.Errorf("DisplayName = %q, want %q", m.DisplayName, "GitHub")
	}

	if _, ok := MetadataFor(ProviderType("not-a-real-provider")); ok {
		t.Error("MetadataFor(unknown) returned ok=true")
	}
}

func TestAllProviderMetadata_ReturnsCopy(t *testing.T) {
	first := AllProviderMetadata()
	if len(first) == 0 {
		t.Fatal("AllProviderMetadata returned empty slice")
	}
	first[0].DisplayName = "mutated"
	second := AllProviderMetadata()
	if second[0].DisplayName == "mutated" {
		t.Error("AllProviderMetadata returned a shared slice; callers can mutate package state")
	}
}
