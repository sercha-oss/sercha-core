package mcp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

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
	}
	fireSearchObserver(obs, want)

	select {
	case got := <-obs.searchEvents:
		if got.UserID != want.UserID || got.Query != want.Query ||
			got.ResultCount != want.ResultCount || got.ClientType != want.ClientType ||
			got.DurationNs != want.DurationNs {
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
