package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// fakeRetrievalObserver is a RetrievalObserver test double that records
// invocations and releases blocked callers through channels.
type fakeRetrievalObserver struct {
	searchEvents   chan driven.SearchCompletedEvent
	documentEvents chan driven.DocumentRetrievedEvent
	// searchInvoked / docInvoked are counters so tests can assert
	// "called exactly once" after draining the channels.
	searchInvoked int32
	docInvoked    int32
	// returnErr is returned from both methods. Errors must not surface
	// to callers.
	returnErr error
	// beforeReturn, if non-nil, is closed by the observer on entry and
	// read-blocks on release so tests can confirm the handler returned
	// before the observer did.
	release chan struct{}
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

// --- handleSearch -----------------------------------------------------------

func TestHandleSearch_ObserverNilDoesNotPanic(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return &domain.SearchResult{
				Query:      query,
				TotalCount: 0,
				Mode:       opts.Mode,
				Results:    []*domain.SearchResultItem{},
			}, nil
		},
	}
	server := &Server{searchService: mockSearch} // retrievalObserver left nil

	body, _ := json.Marshal(searchRequest{Query: "q"})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHandleSearch_ObserverFiresOnceWithPayload(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return &domain.SearchResult{
				Query:      query,
				TotalCount: 2,
				Took:       10 * time.Millisecond,
				Mode:       opts.Mode,
				Results: []*domain.SearchResultItem{
					{DocumentID: "doc-a"},
					{DocumentID: "doc-b"},
				},
			}, nil
		},
	}
	observer := newFakeRetrievalObserver()
	server := &Server{searchService: mockSearch, retrievalObserver: observer}

	body, _ := json.Marshal(searchRequest{Query: "hello"})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	// Attach an auth context so UserID propagates.
	ctx := context.WithValue(req.Context(), authContextKey, &domain.AuthContext{
		UserID: "user-42",
		TeamID: "team-1",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var event driven.SearchCompletedEvent
	select {
	case event = <-observer.searchEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}

	if got := atomic.LoadInt32(&observer.searchInvoked); got != 1 {
		t.Errorf("expected observer invoked 1 time, got %d", got)
	}
	if event.Query != "hello" {
		t.Errorf("event.Query = %q, want %q", event.Query, "hello")
	}
	if event.UserID != "user-42" {
		t.Errorf("event.UserID = %q, want user-42", event.UserID)
	}
	if event.ResultCount != 2 {
		t.Errorf("event.ResultCount = %d, want 2", event.ResultCount)
	}
	if event.ClientType != "http" {
		t.Errorf("event.ClientType = %q, want http", event.ClientType)
	}
	if event.ClientID != "" {
		t.Errorf("event.ClientID = %q, want empty (session-based HTTP)", event.ClientID)
	}
	if event.ClientName != "" {
		t.Errorf("event.ClientName = %q, want empty (session-based HTTP)", event.ClientName)
	}
	if !reflect.DeepEqual(event.DocumentIDs, []string{"doc-a", "doc-b"}) {
		t.Errorf("event.DocumentIDs = %v, want [doc-a doc-b]", event.DocumentIDs)
	}
	if event.DurationNs <= 0 {
		t.Errorf("event.DurationNs should be positive, got %d", event.DurationNs)
	}
}

func TestHandleSearch_ObserverErrorDoesNotAffectResponse(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return &domain.SearchResult{
				Query:      query,
				TotalCount: 0,
				Mode:       opts.Mode,
				Results:    []*domain.SearchResultItem{},
			}, nil
		},
	}
	observer := newFakeRetrievalObserver()
	observer.returnErr = errors.New("boom")
	server := &Server{searchService: mockSearch, retrievalObserver: observer}

	body, _ := json.Marshal(searchRequest{Query: "q"})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	// Wait for async observer so the goroutine actually runs and
	// we can confirm it saw the error path without leaking.
	select {
	case <-observer.searchEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}
}

func TestHandleSearch_ObserverNotInvokedOnServiceError(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return nil, errors.New("search failed")
		},
	}
	observer := newFakeRetrievalObserver()
	server := &Server{searchService: mockSearch, retrievalObserver: observer}

	body, _ := json.Marshal(searchRequest{Query: "q"})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	// Give any (incorrectly) spawned goroutine a chance to run.
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&observer.searchInvoked); got != 0 {
		t.Errorf("observer should not fire on 5xx, invocations: %d", got)
	}
}

func TestHandleSearch_ObserverNotInvokedOnValidationError(t *testing.T) {
	mockSearch := &mockSearchService{}
	observer := newFakeRetrievalObserver()
	server := &Server{searchService: mockSearch, retrievalObserver: observer}

	body, _ := json.Marshal(searchRequest{Query: ""})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	server.handleSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&observer.searchInvoked); got != 0 {
		t.Errorf("observer should not fire on 4xx, invocations: %d", got)
	}
}

func TestHandleSearch_ObserverRunsAsynchronously(t *testing.T) {
	mockSearch := &mockSearchService{
		searchFn: func(ctx context.Context, query string, opts domain.SearchOptions) (*domain.SearchResult, error) {
			return &domain.SearchResult{
				Query:      query,
				TotalCount: 0,
				Mode:       opts.Mode,
				Results:    []*domain.SearchResultItem{},
			}, nil
		},
	}
	observer := newFakeRetrievalObserver()
	observer.release = make(chan struct{})
	server := &Server{searchService: mockSearch, retrievalObserver: observer}

	body, _ := json.Marshal(searchRequest{Query: "q"})
	req := httptest.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	// Handler must return even though observer is blocked on release.
	done := make(chan struct{})
	go func() {
		server.handleSearch(rr, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("handler blocked on observer — observer must run asynchronously")
	}

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Now release the observer and drain.
	close(observer.release)
	select {
	case <-observer.searchEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer never unblocked")
	}
}

// --- handleGetDocument ------------------------------------------------------

func TestHandleGetDocument_ObserverNilDoesNotPanic(t *testing.T) {
	mockDoc := &mockDocumentService{
		getFn: func(ctx context.Context, id string) (*domain.Document, error) {
			return &domain.Document{ID: id, Title: "t"}, nil
		},
	}
	server := &Server{docService: mockDoc}

	req := httptest.NewRequest("GET", "/api/v1/documents/doc-1", nil)
	req.SetPathValue("id", "doc-1")
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHandleGetDocument_ObserverFiresOnceWithPayload(t *testing.T) {
	mockDoc := &mockDocumentService{
		getFn: func(ctx context.Context, id string) (*domain.Document, error) {
			return &domain.Document{ID: id, Title: "t"}, nil
		},
	}
	observer := newFakeRetrievalObserver()
	server := &Server{docService: mockDoc, retrievalObserver: observer}

	req := httptest.NewRequest("GET", "/api/v1/documents/doc-1", nil)
	req.SetPathValue("id", "doc-1")
	ctx := context.WithValue(req.Context(), authContextKey, &domain.AuthContext{
		UserID: "user-7",
		TeamID: "team-1",
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var event driven.DocumentRetrievedEvent
	select {
	case event = <-observer.documentEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}

	if got := atomic.LoadInt32(&observer.docInvoked); got != 1 {
		t.Errorf("expected observer invoked 1 time, got %d", got)
	}
	if event.DocumentID != "doc-1" {
		t.Errorf("event.DocumentID = %q, want doc-1", event.DocumentID)
	}
	if event.UserID != "user-7" {
		t.Errorf("event.UserID = %q, want user-7", event.UserID)
	}
	if event.ClientType != "http" {
		t.Errorf("event.ClientType = %q, want http", event.ClientType)
	}
	if event.ClientID != "" {
		t.Errorf("event.ClientID = %q, want empty (session-based HTTP)", event.ClientID)
	}
	if event.ClientName != "" {
		t.Errorf("event.ClientName = %q, want empty (session-based HTTP)", event.ClientName)
	}
	if event.DurationNs <= 0 {
		t.Errorf("event.DurationNs should be positive, got %d", event.DurationNs)
	}
}

func TestHandleGetDocument_ObserverErrorDoesNotAffectResponse(t *testing.T) {
	mockDoc := &mockDocumentService{
		getFn: func(ctx context.Context, id string) (*domain.Document, error) {
			return &domain.Document{ID: id, Title: "t"}, nil
		},
	}
	observer := newFakeRetrievalObserver()
	observer.returnErr = errors.New("nope")
	server := &Server{docService: mockDoc, retrievalObserver: observer}

	req := httptest.NewRequest("GET", "/api/v1/documents/doc-1", nil)
	req.SetPathValue("id", "doc-1")
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	select {
	case <-observer.documentEvents:
	case <-time.After(2 * time.Second):
		t.Fatal("observer was not invoked within 2s")
	}
}

func TestHandleGetDocument_ObserverNotInvokedOnNotFound(t *testing.T) {
	mockDoc := &mockDocumentService{
		getFn: func(ctx context.Context, id string) (*domain.Document, error) {
			return nil, domain.ErrNotFound
		},
	}
	observer := newFakeRetrievalObserver()
	server := &Server{docService: mockDoc, retrievalObserver: observer}

	req := httptest.NewRequest("GET", "/api/v1/documents/nope", nil)
	req.SetPathValue("id", "nope")
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&observer.docInvoked); got != 0 {
		t.Errorf("observer should not fire on 4xx, invocations: %d", got)
	}
}

func TestHandleGetDocument_ObserverNotInvokedOnMissingID(t *testing.T) {
	observer := newFakeRetrievalObserver()
	server := &Server{retrievalObserver: observer}

	req := httptest.NewRequest("GET", "/api/v1/documents/", nil)
	rr := httptest.NewRecorder()

	server.handleGetDocument(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}

	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&observer.docInvoked); got != 0 {
		t.Errorf("observer should not fire on 400, invocations: %d", got)
	}
}

// --- SetRetrievalObserver ---------------------------------------------------

func TestServer_SetRetrievalObserver(t *testing.T) {
	s := &Server{}
	if s.retrievalObserver != nil {
		t.Fatalf("expected default nil observer")
	}
	obs := newFakeRetrievalObserver()
	s.SetRetrievalObserver(obs)
	if s.retrievalObserver == nil {
		t.Fatalf("SetRetrievalObserver did not install observer")
	}
	s.SetRetrievalObserver(nil)
	if s.retrievalObserver != nil {
		t.Fatalf("SetRetrievalObserver(nil) did not clear observer")
	}
}
