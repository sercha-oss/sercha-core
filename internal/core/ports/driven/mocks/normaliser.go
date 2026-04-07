package mocks

import (
	"github.com/custodia-labs/sercha-core/internal/core/ports/driven"
)

// MockNormaliser is a mock implementation of Normaliser for testing
type MockNormaliser struct {
	SupportedTypesFn func() []string
	PriorityFn       func() int
	NormaliseFn      func(content string, mimeType string) string
}

func NewMockNormaliser() *MockNormaliser {
	return &MockNormaliser{}
}

func (m *MockNormaliser) Normalise(content string, mimeType string) string {
	if m.NormaliseFn != nil {
		return m.NormaliseFn(content, mimeType)
	}
	return content
}

func (m *MockNormaliser) SupportedTypes() []string {
	if m.SupportedTypesFn != nil {
		return m.SupportedTypesFn()
	}
	return []string{"text/plain", "text/html"}
}

func (m *MockNormaliser) Priority() int {
	if m.PriorityFn != nil {
		return m.PriorityFn()
	}
	return 100
}

// MockNormaliserRegistry is a mock implementation of NormaliserRegistry for testing
type MockNormaliserRegistry struct {
	GetFn      func(mimeType string) driven.Normaliser
	GetAllFn   func(mimeType string) []driven.Normaliser
	RegisterFn func(normaliser driven.Normaliser)
	normaliser driven.Normaliser
}

func NewMockNormaliserRegistry() *MockNormaliserRegistry {
	return &MockNormaliserRegistry{
		normaliser: NewMockNormaliser(),
	}
}

func (m *MockNormaliserRegistry) Get(mimeType string) driven.Normaliser {
	if m.GetFn != nil {
		return m.GetFn(mimeType)
	}
	return m.normaliser
}

func (m *MockNormaliserRegistry) GetAll(mimeType string) []driven.Normaliser {
	if m.GetAllFn != nil {
		return m.GetAllFn(mimeType)
	}
	if m.normaliser != nil {
		return []driven.Normaliser{m.normaliser}
	}
	return nil
}

func (m *MockNormaliserRegistry) Register(normaliser driven.Normaliser) {
	if m.RegisterFn != nil {
		m.RegisterFn(normaliser)
	}
	m.normaliser = normaliser
}

// List returns all registered MIME types
func (m *MockNormaliserRegistry) List() []string {
	if m.normaliser != nil {
		return m.normaliser.SupportedTypes()
	}
	return []string{}
}

// SetNormaliser sets the normaliser returned by Get
func (m *MockNormaliserRegistry) SetNormaliser(n driven.Normaliser) {
	m.normaliser = n
}
