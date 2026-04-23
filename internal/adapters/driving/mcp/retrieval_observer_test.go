package mcp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// fakeRetrievalObserver is a test double for driven.RetrievalObserver.
type fakeRetrievalObserver struct {
	searchEvents   chan driven.SearchCompletedEvent
	documentEvents chan driven.DocumentRetrievedEvent
	searchInvoked  int32
	docInvoked     int32
	returnErr      error
	release        chan struct{}
}

func newFakeRetrievalObserver() *fakeRetrievalObserver {
	return &fakeRetrievalObserver{
		searchEvents:   make(chan driven.SearchCompletedEvent, 1),
		documentEvents: make(chan driven.DocumentRetrievedEvent, 1),
	}
}

func (f *fakeRetrievalObserver) OnSearchCompleted(ctx context.Context, event driven.SearchCompletedEvent) error {
	atomic.AddInt32(&f.searchInvoked, 1)
	if f.release != nil {
		<-f.release
	}
	f.searchEvents <- event
	return f.returnErr
}

func (f *fakeRetrievalObserver) OnDocumentRetrieved(ctx context.Context, event driven.DocumentRetrievedEvent) error {
	atomic.AddInt32(&f.docInvoked, 1)
	if f.release != nil {
		<-f.release
	}
	f.documentEvents <- event
	return f.returnErr
}

// --- fireSearchObserver -----------------------------------------------------

func TestFireSearchObserver_NilIsNoop(t *testing.T) {
	// Must not panic and must not spawn anything observable.
	fireSearchObserver(nil, driven.SearchCompletedEvent{})
}

func TestFireSearchObserver_FiresExactlyOnce(t *testing.T) {
	obs := newFakeRetrievalObserver()
	want := driven.SearchCompletedEvent{
		UserID:      "u1",
		Query:       "q",
		DocumentIDs: []string{"a", "b"},
		ResultCount: 2,
		DurationNs:  100,
		ClientType:  "mcp",
		ClientID:    "client-1",
		ClientName:  "Claude Desktop",
	}
	fireSearchObserver(obs, want)

	select {
	case got := <-obs.searchEvents:
		if got.UserID != want.UserID || got.Query != want.Query ||
			got.ResultCount != want.ResultCount || got.ClientType != want.ClientType ||
			got.DurationNs != want.DurationNs ||
			got.ClientID != want.ClientID || got.ClientName != want.ClientName {
			t.Errorf("event mismatch: got %+v want %+v", got, want)
		}
		if len(got.DocumentIDs) != 2 || got.DocumentIDs[0] != "a" || got.DocumentIDs[1] != "b" {
			t.Errorf("DocumentIDs mismatch: got %v", got.DocumentIDs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}

	if got := atomic.LoadInt32(&obs.searchInvoked); got != 1 {
		t.Errorf("expected 1 invocation, got %d", got)
	}
}

func TestFireSearchObserver_ErrorIsSwallowed(t *testing.T) {
	obs := newFakeRetrievalObserver()
	obs.returnErr = errors.New("downstream failure")
	fireSearchObserver(obs, driven.SearchCompletedEvent{ClientType: "mcp"})

	// Caller path returns immediately. Error must not surface anywhere.
	// Drain the channel to ensure the goroutine ran with the error.
	select {
	case <-obs.searchEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}
}

func TestFireSearchObserver_IsAsync(t *testing.T) {
	obs := newFakeRetrievalObserver()
	obs.release = make(chan struct{})

	done := make(chan struct{})
	go func() {
		fireSearchObserver(obs, driven.SearchCompletedEvent{ClientType: "mcp"})
		close(done)
	}()

	// fireSearchObserver must return even though the observer is blocked.
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("fireSearchObserver blocked — must be asynchronous")
	}

	close(obs.release)
	select {
	case <-obs.searchEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer never unblocked")
	}
}

// --- fireDocumentObserver ---------------------------------------------------

func TestFireDocumentObserver_NilIsNoop(t *testing.T) {
	fireDocumentObserver(nil, driven.DocumentRetrievedEvent{})
}

func TestFireDocumentObserver_FiresExactlyOnce(t *testing.T) {
	obs := newFakeRetrievalObserver()
	want := driven.DocumentRetrievedEvent{
		UserID:     "u1",
		DocumentID: "doc-42",
		DurationNs: 500,
		ClientType: "mcp",
		ClientID:   "client-1",
		ClientName: "Cursor",
	}
	fireDocumentObserver(obs, want)

	select {
	case got := <-obs.documentEvents:
		if got != want {
			t.Errorf("event mismatch: got %+v want %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}

	if got := atomic.LoadInt32(&obs.docInvoked); got != 1 {
		t.Errorf("expected 1 invocation, got %d", got)
	}
}

func TestFireDocumentObserver_ErrorIsSwallowed(t *testing.T) {
	obs := newFakeRetrievalObserver()
	obs.returnErr = errors.New("nope")
	fireDocumentObserver(obs, driven.DocumentRetrievedEvent{ClientType: "mcp"})

	select {
	case <-obs.documentEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}
}

func TestFireDocumentObserver_IsAsync(t *testing.T) {
	obs := newFakeRetrievalObserver()
	obs.release = make(chan struct{})

	done := make(chan struct{})
	go func() {
		fireDocumentObserver(obs, driven.DocumentRetrievedEvent{ClientType: "mcp"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("fireDocumentObserver blocked — must be asynchronous")
	}

	close(obs.release)
	select {
	case <-obs.documentEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer never unblocked")
	}
}

// --- server construction ----------------------------------------------------

func TestNewMCPServer_NilObserver(t *testing.T) {
	// Server must construct cleanly with a nil RetrievalObserver.
	srv := NewMCPServer(MCPServerConfig{
		Version: "test",
	})
	if srv == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

func TestNewMCPServer_WithObserver(t *testing.T) {
	srv := NewMCPServer(MCPServerConfig{
		Version:           "test",
		RetrievalObserver: newFakeRetrievalObserver(),
	})
	if srv == nil {
		t.Fatal("NewMCPServer returned nil")
	}
}

// --- clientIdentityFromTokenInfo -------------------------------------------

func TestClientIdentityFromTokenInfo_NilReturnsEmpty(t *testing.T) {
	id, name := clientIdentityFromTokenInfo(nil)
	if id != "" || name != "" {
		t.Errorf("nil tokenInfo should yield empty strings, got id=%q name=%q", id, name)
	}
}

func TestClientIdentityFromTokenInfo_MissingKeys(t *testing.T) {
	info := &auth.TokenInfo{Extra: map[string]any{}}
	id, name := clientIdentityFromTokenInfo(info)
	if id != "" || name != "" {
		t.Errorf("missing keys should yield empty strings, got id=%q name=%q", id, name)
	}
}

func TestClientIdentityFromTokenInfo_NilExtraIsSafe(t *testing.T) {
	// nil map reads are defined-behaviour in Go (return zero value), but
	// this test pins that so nobody later "fixes" the helper by adding a
	// defensive check that would silently change semantics.
	info := &auth.TokenInfo{Extra: nil}
	id, name := clientIdentityFromTokenInfo(info)
	if id != "" || name != "" {
		t.Errorf("nil Extra should yield empty strings, got id=%q name=%q", id, name)
	}
}

func TestClientIdentityFromTokenInfo_PopulatedStrings(t *testing.T) {
	info := &auth.TokenInfo{Extra: map[string]any{
		"client_id":   "oauth-client-42",
		"client_name": "Claude Desktop",
	}}
	id, name := clientIdentityFromTokenInfo(info)
	if id != "oauth-client-42" {
		t.Errorf("client_id: got %q, want oauth-client-42", id)
	}
	if name != "Claude Desktop" {
		t.Errorf("client_name: got %q, want Claude Desktop", name)
	}
}

func TestClientIdentityFromTokenInfo_NonStringValuesDoNotPanic(t *testing.T) {
	// Anyone writing to TokenInfo.Extra could stuff a non-string in there.
	// The type assertion's comma-ok form must drop it cleanly.
	info := &auth.TokenInfo{Extra: map[string]any{
		"client_id":   12345, // int, not string
		"client_name": []string{"a", "b"},
	}}
	id, name := clientIdentityFromTokenInfo(info)
	if id != "" {
		t.Errorf("non-string client_id: got %q, want empty", id)
	}
	if name != "" {
		t.Errorf("non-string client_name: got %q, want empty", name)
	}
}

func TestClientIdentityFromTokenInfo_PartialPopulation(t *testing.T) {
	// client_id present, client_name absent — realistic state today since
	// the verifier only populates client_id.
	info := &auth.TokenInfo{Extra: map[string]any{
		"client_id": "oauth-client-42",
	}}
	id, name := clientIdentityFromTokenInfo(info)
	if id != "oauth-client-42" {
		t.Errorf("client_id: got %q, want oauth-client-42", id)
	}
	if name != "" {
		t.Errorf("client_name should be empty, got %q", name)
	}
}
