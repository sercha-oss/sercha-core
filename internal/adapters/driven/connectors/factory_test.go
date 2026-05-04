package connectors

import (
	"context"
	"errors"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// fakeTokenProviderFactory records every Create call and returns a
// pre-configured stub provider (or error). Used to assert whether the
// Factory consulted the token-provider factory or skipped it.
type fakeTokenProviderFactory struct {
	calls   []string
	stubFor map[string]driven.TokenProvider
	errFor  map[string]error
}

func (f *fakeTokenProviderFactory) Create(_ context.Context, connectionID string) (driven.TokenProvider, error) {
	f.calls = append(f.calls, connectionID)
	if err, ok := f.errFor[connectionID]; ok {
		return nil, err
	}
	if tp, ok := f.stubFor[connectionID]; ok {
		return tp, nil
	}
	return &fakeTokenProvider{}, nil
}

func (f *fakeTokenProviderFactory) CreateFromConnection(_ context.Context, _ *domain.Connection) (driven.TokenProvider, error) {
	return &fakeTokenProvider{}, nil
}

// fakeTokenProvider is a stub satisfying driven.TokenProvider for tests
// that only need a non-nil reference.
type fakeTokenProvider struct{}

func (p *fakeTokenProvider) GetAccessToken(_ context.Context) (string, error) {
	return "fake-token", nil
}
func (p *fakeTokenProvider) GetCredentials(_ context.Context) (*domain.Credentials, error) {
	return &domain.Credentials{AuthMethod: domain.AuthMethodOAuth2, AccessToken: "fake-token"}, nil
}
func (p *fakeTokenProvider) AuthMethod() domain.AuthMethod  { return domain.AuthMethodOAuth2 }
func (p *fakeTokenProvider) IsValid(_ context.Context) bool { return true }

// fakeBuilder records what TokenProvider it received in Build.
type fakeBuilder struct {
	providerType     domain.ProviderType
	receivedToken    driven.TokenProvider
	receivedCalled   bool
	connectorToBuild driven.Connector
	buildErr         error
}

func (b *fakeBuilder) Type() domain.ProviderType { return b.providerType }

func (b *fakeBuilder) Build(_ context.Context, tp driven.TokenProvider, _ string) (driven.Connector, error) {
	b.receivedCalled = true
	b.receivedToken = tp
	if b.buildErr != nil {
		return nil, b.buildErr
	}
	return b.connectorToBuild, nil
}

func (b *fakeBuilder) SupportsOAuth() bool              { return false }
func (b *fakeBuilder) OAuthConfig() *driven.OAuthConfig { return nil }
func (b *fakeBuilder) SupportsContainerSelection() bool { return false }

// fakeConnector — minimal Connector that lets us assert Build returned
// our expected instance.
type fakeConnector struct{ providerType domain.ProviderType }

func (c *fakeConnector) Type() domain.ProviderType                  { return c.providerType }
func (c *fakeConnector) ValidateConfig(_ domain.SourceConfig) error { return nil }
func (c *fakeConnector) FetchChanges(_ context.Context, _ *domain.Source, _ string) ([]*domain.Change, string, error) {
	return nil, "", nil
}
func (c *fakeConnector) FetchDocument(_ context.Context, _ *domain.Source, _ string) (*domain.Document, string, error) {
	return nil, "", nil
}
func (c *fakeConnector) TestConnection(_ context.Context, _ *domain.Source) error { return nil }
func (c *fakeConnector) ReconciliationScopes() []string                           { return nil }
func (c *fakeConnector) Inventory(_ context.Context, _ *domain.Source, _ string) ([]string, error) {
	return nil, driven.ErrInventoryNotSupported
}
func (c *fakeConnector) RESTClient() driven.RESTClient { return nil }

// TestFactoryCreate_DelegatedSourceResolvesTokenProvider verifies that
// when source.ConnectionID is non-empty, the Factory looks up a
// TokenProvider via tokenProviderFactory.Create and passes it to the
// Builder. Regression test for the existing delegated-OAuth flow.
func TestFactoryCreate_DelegatedSourceResolvesTokenProvider(t *testing.T) {
	tpf := &fakeTokenProviderFactory{stubFor: map[string]driven.TokenProvider{}}
	factory := NewFactory(tpf)

	conn := &fakeConnector{providerType: "test-delegated"}
	builder := &fakeBuilder{providerType: "test-delegated", connectorToBuild: conn}
	factory.Register(builder)

	source := &domain.Source{
		ID:           "src-1",
		ProviderType: "test-delegated",
		ConnectionID: "conn-abc",
	}

	got, err := factory.Create(context.Background(), source, "")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if got != conn {
		t.Errorf("Create() returned wrong connector instance")
	}

	if len(tpf.calls) != 1 || tpf.calls[0] != "conn-abc" {
		t.Errorf("expected exactly one tokenProviderFactory.Create(\"conn-abc\") call, got %v", tpf.calls)
	}
	if !builder.receivedCalled {
		t.Fatal("Builder.Build was not called")
	}
	if builder.receivedToken == nil {
		t.Error("Builder.Build received nil TokenProvider for delegated source — expected non-nil")
	}
}

// TestFactoryCreate_AppOnlySourceSkipsTokenProviderLookup verifies the
// new behaviour: when source.ConnectionID is empty, the Factory does NOT
// consult tokenProviderFactory.Create and passes nil to the Builder. This
// is the path used by deployment-level credential connectors (app-only
// auth, service principals) whose Builders capture credentials at
// registration time rather than per-source.
func TestFactoryCreate_AppOnlySourceSkipsTokenProviderLookup(t *testing.T) {
	tpf := &fakeTokenProviderFactory{}
	factory := NewFactory(tpf)

	conn := &fakeConnector{providerType: "test-app-only"}
	builder := &fakeBuilder{providerType: "test-app-only", connectorToBuild: conn}
	factory.Register(builder)

	source := &domain.Source{
		ID:           "src-2",
		ProviderType: "test-app-only",
		ConnectionID: "", // explicit: app-only sources have no per-source connection
	}

	got, err := factory.Create(context.Background(), source, "")
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if got != conn {
		t.Errorf("Create() returned wrong connector instance")
	}

	if len(tpf.calls) != 0 {
		t.Errorf("expected zero tokenProviderFactory.Create calls for empty ConnectionID, got %v", tpf.calls)
	}
	if !builder.receivedCalled {
		t.Fatal("Builder.Build was not called")
	}
	if builder.receivedToken != nil {
		t.Errorf("Builder.Build received non-nil TokenProvider for empty ConnectionID — expected nil, got %T", builder.receivedToken)
	}
}

// TestFactoryCreate_TokenProviderFactoryErrorPropagates verifies that a
// failure inside tokenProviderFactory.Create surfaces as an error from
// Factory.Create when ConnectionID is non-empty.
func TestFactoryCreate_TokenProviderFactoryErrorPropagates(t *testing.T) {
	wantErr := errors.New("upstream connection store unreachable")
	tpf := &fakeTokenProviderFactory{
		errFor: map[string]error{"conn-bad": wantErr},
	}
	factory := NewFactory(tpf)

	builder := &fakeBuilder{providerType: "test-delegated"}
	factory.Register(builder)

	source := &domain.Source{
		ProviderType: "test-delegated",
		ConnectionID: "conn-bad",
	}

	_, err := factory.Create(context.Background(), source, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped %v, got %v", wantErr, err)
	}
	if builder.receivedCalled {
		t.Error("Builder.Build should not have been called when token-provider lookup fails")
	}
}

// TestFactoryCreate_UnknownProviderType verifies that an unregistered
// provider type returns ErrUnsupportedProvider regardless of whether
// ConnectionID is set. Documents existing behaviour around the new branch.
func TestFactoryCreate_UnknownProviderType(t *testing.T) {
	tpf := &fakeTokenProviderFactory{}
	factory := NewFactory(tpf)

	source := &domain.Source{
		ProviderType: "never-registered",
		ConnectionID: "",
	}

	_, err := factory.Create(context.Background(), source, "")
	if err == nil {
		t.Fatal("expected ErrUnsupportedProvider, got nil")
	}
	if !errors.Is(err, domain.ErrUnsupportedProvider) {
		t.Errorf("expected ErrUnsupportedProvider, got %v", err)
	}

	if len(tpf.calls) != 0 {
		t.Errorf("expected no token-provider lookups for unknown provider, got %v", tpf.calls)
	}
}

// TestFactoryCreate_BuilderErrorPropagates verifies Build errors surface
// unmodified through Create, for the app-only path (nil token provider
// passed to Builder).
func TestFactoryCreate_BuilderErrorPropagates(t *testing.T) {
	wantErr := errors.New("builder rejected nil token provider")
	tpf := &fakeTokenProviderFactory{}
	factory := NewFactory(tpf)

	builder := &fakeBuilder{providerType: "test-strict", buildErr: wantErr}
	factory.Register(builder)

	source := &domain.Source{
		ProviderType: "test-strict",
		ConnectionID: "", // app-only path: nil provider passed to Builder
	}

	_, err := factory.Create(context.Background(), source, "")
	if err == nil {
		t.Fatal("expected error from Builder, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped %v, got %v", wantErr, err)
	}
}
