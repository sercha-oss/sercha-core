package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// staticCred is a test-only DBCredentialProvider that returns a fixed DSN.
// We define it as a function type so the file does not need a struct receiver.
type staticCred string

func (s staticCred) Resolve(_ context.Context) (string, error) {
	return string(s), nil
}

// Compile-time check that staticCred satisfies the port. If the port shape
// changes, this test file fails to compile, surfacing the breakage early.
var _ driven.DBCredentialProvider = staticCred("")

// TestConnect_NilProviderReturnsClearError ensures Connect refuses a nil
// provider with an explanatory message, rather than panicking.
func TestConnect_NilProviderReturnsClearError(t *testing.T) {
	ctx := context.Background()
	_, err := Connect(ctx, nil, DefaultConfig())
	if err == nil {
		t.Fatal("expected error from Connect with nil provider, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected error to mention nil provider, got %v", err)
	}
}

// TestConnect_SSLGuard_FiresWhenDevUnset verifies the sslmode=disable guard:
// when the DSN contains sslmode=disable and SERCHA_DEV is unset, Connect must
// refuse with the documented error.
func TestConnect_SSLGuard_FiresWhenDevUnset(t *testing.T) {
	t.Setenv("SERCHA_DEV", "") // explicitly clear, regardless of host env

	dsn := "postgres://u:p@127.0.0.1:1/db?sslmode=disable"
	_, err := Connect(context.Background(), staticCred(dsn), DefaultConfig())
	if err == nil {
		t.Fatal("expected SSL guard error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "sslmode=disable not allowed unless SERCHA_DEV=1") {
		t.Errorf("expected SSL guard error, got %v", err)
	}
}

// TestConnect_SSLGuard_PassesWhenDevSet verifies that the SSL guard does not
// fire when SERCHA_DEV=1 is set. The connection ultimately fails (we use a
// bogus host) — that's expected; we just need to confirm the failure is NOT
// the guard message.
func TestConnect_SSLGuard_PassesWhenDevSet(t *testing.T) {
	t.Setenv("SERCHA_DEV", "1")

	// Use an unroutable port so Ping fails fast. The exact failure mode is
	// not what we're asserting — we're asserting that whatever does fail,
	// it is NOT the SSL guard.
	dsn := "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"
	_, err := Connect(context.Background(), staticCred(dsn), DefaultConfig())
	if err == nil {
		// Highly unlikely on a normal CI box, but if for some reason the
		// connection actually succeeded, the guard certainly didn't fire.
		t.Log("Connect unexpectedly succeeded; guard clearly did not fire")
		return
	}
	if strings.Contains(err.Error(), "sslmode=disable not allowed") {
		t.Errorf("SSL guard fired with SERCHA_DEV=1 set: %v", err)
	}
}

// TestConnect_SSLGuard_PassesWhenSSLModeRequire ensures a DSN that asks for
// sslmode=require is not blocked by the guard, even with SERCHA_DEV unset.
func TestConnect_SSLGuard_PassesWhenSSLModeRequire(t *testing.T) {
	t.Setenv("SERCHA_DEV", "")

	dsn := "postgres://u:p@127.0.0.1:1/db?sslmode=require&connect_timeout=1"
	_, err := Connect(context.Background(), staticCred(dsn), DefaultConfig())
	if err == nil {
		t.Log("Connect unexpectedly succeeded; guard clearly did not fire")
		return
	}
	if strings.Contains(err.Error(), "sslmode=disable not allowed") {
		t.Errorf("SSL guard fired for sslmode=require: %v", err)
	}
}

// TestConnect_PropagatesProviderError ensures that when the credential
// provider fails, the error is wrapped and returned (not silently masked).
func TestConnect_PropagatesProviderError(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	provider := NewEnvDBCredential()

	_, err := Connect(context.Background(), provider, DefaultConfig())
	if err == nil {
		t.Fatal("expected error from provider, got nil")
	}
	if !strings.Contains(err.Error(), "resolve db credential") {
		t.Errorf("expected wrapped resolve error, got %v", err)
	}
}
