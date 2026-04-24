package mocks

import (
	"context"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// MockConnector is a mock implementation of Connector for testing
type MockConnector struct {
	TypeFn                 func() domain.ProviderType
	ValidateConfigFn       func(config domain.SourceConfig) error
	TestConnectionFn       func(ctx context.Context, source *domain.Source) error
	FetchChangesFn         func(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error)
	FetchDocumentFn        func(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error)
	ReconciliationScopesFn func() []string
	InventoryFn            func(ctx context.Context, source *domain.Source, scope string) ([]string, error)
}

func NewMockConnector() *MockConnector {
	return &MockConnector{}
}

func (m *MockConnector) Type() domain.ProviderType {
	if m.TypeFn != nil {
		return m.TypeFn()
	}
	return domain.ProviderTypeGitHub
}

func (m *MockConnector) ValidateConfig(config domain.SourceConfig) error {
	if m.ValidateConfigFn != nil {
		return m.ValidateConfigFn(config)
	}
	return nil
}

func (m *MockConnector) TestConnection(ctx context.Context, source *domain.Source) error {
	if m.TestConnectionFn != nil {
		return m.TestConnectionFn(ctx, source)
	}
	return nil
}

func (m *MockConnector) FetchChanges(ctx context.Context, source *domain.Source, cursor string) ([]*domain.Change, string, error) {
	if m.FetchChangesFn != nil {
		return m.FetchChangesFn(ctx, source, cursor)
	}
	return nil, "", nil
}

func (m *MockConnector) FetchDocument(ctx context.Context, source *domain.Source, externalID string) (*domain.Document, string, error) {
	if m.FetchDocumentFn != nil {
		return m.FetchDocumentFn(ctx, source, externalID)
	}
	return nil, "", nil
}

func (m *MockConnector) ReconciliationScopes() []string {
	if m.ReconciliationScopesFn != nil {
		return m.ReconciliationScopesFn()
	}
	return nil
}

func (m *MockConnector) Inventory(ctx context.Context, source *domain.Source, scope string) ([]string, error) {
	if m.InventoryFn != nil {
		return m.InventoryFn(ctx, source, scope)
	}
	return nil, driven.ErrInventoryNotSupported
}

// MockConnectorFactory is a mock implementation of ConnectorFactory for testing
type MockConnectorFactory struct {
	RegisterFn       func(builder interface{})
	CreateFn         func(ctx context.Context, source *domain.Source) (*MockConnector, error)
	SupportedTypesFn func() []domain.ProviderType
	GetBuilderFn     func(providerType domain.ProviderType) (interface{}, error)
	SupportsOAuthFn  func(providerType domain.ProviderType) bool
	GetOAuthConfigFn func(providerType domain.ProviderType) interface{}
	connector        *MockConnector
}

func NewMockConnectorFactory() *MockConnectorFactory {
	return &MockConnectorFactory{
		connector: NewMockConnector(),
	}
}

func (m *MockConnectorFactory) Register(builder interface{}) {
	if m.RegisterFn != nil {
		m.RegisterFn(builder)
	}
}

func (m *MockConnectorFactory) Create(ctx context.Context, source *domain.Source) (*MockConnector, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, source)
	}
	return m.connector, nil
}

func (m *MockConnectorFactory) SupportedTypes() []domain.ProviderType {
	if m.SupportedTypesFn != nil {
		return m.SupportedTypesFn()
	}
	return []domain.ProviderType{domain.ProviderTypeGitHub}
}

func (m *MockConnectorFactory) GetBuilder(providerType domain.ProviderType) (interface{}, error) {
	if m.GetBuilderFn != nil {
		return m.GetBuilderFn(providerType)
	}
	return nil, nil
}

func (m *MockConnectorFactory) SupportsOAuth(providerType domain.ProviderType) bool {
	if m.SupportsOAuthFn != nil {
		return m.SupportsOAuthFn(providerType)
	}
	return false
}

func (m *MockConnectorFactory) GetOAuthConfig(providerType domain.ProviderType) interface{} {
	if m.GetOAuthConfigFn != nil {
		return m.GetOAuthConfigFn(providerType)
	}
	return nil
}

// SetConnector sets the connector returned by Create
func (m *MockConnectorFactory) SetConnector(c *MockConnector) {
	m.connector = c
}
