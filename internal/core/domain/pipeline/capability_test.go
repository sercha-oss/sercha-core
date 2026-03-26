package pipeline

import (
	"testing"
)

func TestCapabilitySet_AddAndGet(t *testing.T) {
	cs := NewCapabilitySet()

	// Add capabilities
	cs.Add(CapabilityEmbedder, "openai-embed", "mock-embedder")
	cs.Add(CapabilityLLM, "openai-llm", "mock-llm")
	cs.Add(CapabilityEmbedder, "cohere-embed", "mock-cohere")

	// Get first embedder
	inst, ok := cs.Get(CapabilityEmbedder)
	if !ok {
		t.Error("expected embedder to be found")
	}
	if inst.ID != "openai-embed" {
		t.Errorf("expected first embedder ID 'openai-embed', got %s", inst.ID)
	}

	// Get specific embedder by ID
	inst, ok = cs.GetByID(CapabilityEmbedder, "cohere-embed")
	if !ok {
		t.Error("expected cohere embedder to be found")
	}
	if inst.ID != "cohere-embed" {
		t.Errorf("expected ID 'cohere-embed', got %s", inst.ID)
	}

	// Get non-existent
	_, ok = cs.Get(CapabilityGraphStore)
	if ok {
		t.Error("expected graph store to not be found")
	}
}

func TestCapabilitySet_GetAll(t *testing.T) {
	cs := NewCapabilitySet()

	cs.Add(CapabilityEmbedder, "embed-1", "mock1")
	cs.Add(CapabilityEmbedder, "embed-2", "mock2")
	cs.Add(CapabilityLLM, "llm-1", "mock3")

	embedders := cs.GetAll(CapabilityEmbedder)
	if len(embedders) != 2 {
		t.Errorf("expected 2 embedders, got %d", len(embedders))
	}

	llms := cs.GetAll(CapabilityLLM)
	if len(llms) != 1 {
		t.Errorf("expected 1 LLM, got %d", len(llms))
	}

	stores := cs.GetAll(CapabilityVectorStore)
	if len(stores) != 0 {
		t.Errorf("expected 0 vector stores, got %d", len(stores))
	}
}

func TestCapabilitySet_Has(t *testing.T) {
	cs := NewCapabilitySet()

	cs.Add(CapabilityEmbedder, "embed-1", "mock")

	if !cs.Has(CapabilityEmbedder) {
		t.Error("expected Has(embedder) to be true")
	}
	if cs.Has(CapabilityLLM) {
		t.Error("expected Has(llm) to be false")
	}
}

func TestCapabilitySet_Types(t *testing.T) {
	cs := NewCapabilitySet()

	cs.Add(CapabilityEmbedder, "e1", "m1")
	cs.Add(CapabilityLLM, "l1", "m2")
	cs.Add(CapabilityVectorStore, "v1", "m3")

	types := cs.Types()
	if len(types) != 3 {
		t.Errorf("expected 3 types, got %d", len(types))
	}
}
