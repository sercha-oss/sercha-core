package microsoft

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// stubTokenProvider is a test implementation of TokenProvider
type stubTokenProvider struct {
	token string
	err   error
}

func (s *stubTokenProvider) GetAccessToken(ctx context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.token == "" {
		return "test-token", nil
	}
	return s.token, nil
}

func (s *stubTokenProvider) GetCredentials(ctx context.Context) (*domain.Credentials, error) {
	return nil, nil
}

func (s *stubTokenProvider) AuthMethod() domain.AuthMethod {
	return domain.AuthMethodOAuth2
}

func (s *stubTokenProvider) IsValid(ctx context.Context) bool {
	return true
}

func TestClient_GetMe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me" {
			t.Errorf("Path = %q, want /v1.0/me", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:                "user-123",
			DisplayName:       "John Doe",
			UserPrincipalName: "john@example.com",
			Mail:              "john.doe@example.com",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("ID = %q, want user-123", user.ID)
	}

	if user.DisplayName != "John Doe" {
		t.Errorf("DisplayName = %q, want John Doe", user.DisplayName)
	}

	if user.UserPrincipalName != "john@example.com" {
		t.Errorf("UserPrincipalName = %q, want john@example.com", user.UserPrincipalName)
	}
}

func TestClient_GetDelta(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me/drive/root/delta" {
			t.Errorf("Path = %q, want /v1.0/me/drive/root/delta", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DeltaResponse{
			Value: []DriveItem{
				{
					ID:                   "item-123",
					Name:                 "test.txt",
					Size:                 1024,
					WebURL:               "https://onedrive.com/test.txt",
					CreatedDateTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					LastModifiedDateTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					File: &FileFacet{
						MimeType: "text/plain",
					},
				},
			},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=abc123",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	delta, err := client.GetDelta(context.Background(), "")
	if err != nil {
		t.Fatalf("GetDelta() error = %v", err)
	}

	if len(delta.Value) != 1 {
		t.Errorf("Value length = %d, want 1", len(delta.Value))
	}

	if delta.Value[0].ID != "item-123" {
		t.Errorf("Item ID = %q, want item-123", delta.Value[0].ID)
	}

	if delta.Value[0].Name != "test.txt" {
		t.Errorf("Item Name = %q, want test.txt", delta.Value[0].Name)
	}

	if !delta.Value[0].IsFile() {
		t.Error("IsFile() = false, want true")
	}

	if delta.DeltaLink == "" {
		t.Error("DeltaLink is empty, want non-empty")
	}
}

func TestClient_GetDelta_WithDeltaLink(t *testing.T) {
	deltaLinkChecked := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that the delta link path is used
		if strings.Contains(r.URL.Path, "/delta") && r.URL.Query().Get("token") == "abc123" {
			deltaLinkChecked = true
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DeltaResponse{
			Value:     []DriveItem{},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=xyz789",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	deltaLink := ts.URL + "/v1.0/me/drive/root/delta?token=abc123"
	_, err := client.GetDelta(context.Background(), deltaLink)
	if err != nil {
		t.Fatalf("GetDelta() error = %v", err)
	}

	if !deltaLinkChecked {
		t.Error("Delta link was not used in request")
	}
}

func TestClient_GetDelta_Pagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		nextLink := "http://" + r.Host + "/v1.0/me/drive/root/delta?skipToken=page2"
		_ = json.NewEncoder(w).Encode(DeltaResponse{
			Value: []DriveItem{
				{ID: "item-1", Name: "file1.txt"},
			},
			NextLink: nextLink,
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	delta, err := client.GetDelta(context.Background(), "")
	if err != nil {
		t.Fatalf("GetDelta() error = %v", err)
	}

	if delta.NextLink == "" {
		t.Error("NextLink is empty, want non-empty for pagination")
	}

	if !strings.Contains(delta.NextLink, "skipToken=page2") {
		t.Errorf("NextLink = %q, want to contain skipToken=page2", delta.NextLink)
	}
}

func TestClient_GetDriveItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me/drive/items/folder-123/children" {
			t.Errorf("Path = %q, want /v1.0/me/drive/items/folder-123/children", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DriveItemsResponse{
			Value: []DriveItem{
				{
					ID:   "child-1",
					Name: "subfolder",
					Folder: &FolderFacet{
						ChildCount: 5,
					},
				},
				{
					ID:   "child-2",
					Name: "document.pdf",
					File: &FileFacet{
						MimeType: "application/pdf",
					},
				},
			},
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	resp, err := client.GetDriveItems(context.Background(), "folder-123")
	if err != nil {
		t.Fatalf("GetDriveItems() error = %v", err)
	}

	if len(resp.Value) != 2 {
		t.Errorf("Value length = %d, want 2", len(resp.Value))
	}

	if !resp.Value[0].IsFolder() {
		t.Error("Item 0 IsFolder() = false, want true")
	}

	if !resp.Value[1].IsFile() {
		t.Error("Item 1 IsFile() = false, want true")
	}
}

func TestClient_GetDriveItem(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me/drive/items/item-123" {
			t.Errorf("Path = %q, want /v1.0/me/drive/items/item-123", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DriveItem{
			ID:   "item-123",
			Name: "test.txt",
			Size: 2048,
			File: &FileFacet{
				MimeType: "text/plain",
			},
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	item, err := client.GetDriveItem(context.Background(), "item-123")
	if err != nil {
		t.Fatalf("GetDriveItem() error = %v", err)
	}

	if item.ID != "item-123" {
		t.Errorf("ID = %q, want item-123", item.ID)
	}

	if item.Size != 2048 {
		t.Errorf("Size = %d, want 2048", item.Size)
	}
}

func TestClient_GetDriveItemContent(t *testing.T) {
	expectedContent := "Hello, OneDrive!"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me/drive/items/item-123/content" {
			t.Errorf("Path = %q, want /v1.0/me/drive/items/item-123/content", r.URL.Path)
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(expectedContent))
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	content, err := client.GetDriveItemContent(context.Background(), "item-123")
	if err != nil {
		t.Fatalf("GetDriveItemContent() error = %v", err)
	}

	if string(content) != expectedContent {
		t.Errorf("Content = %q, want %q", string(content), expectedContent)
	}
}

func TestClient_GetDriveItemContent_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error: &ErrorDetail{
				Code:    "itemNotFound",
				Message: "The item does not exist",
			},
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetDriveItemContent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("GetDriveItemContent() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want to contain '404'", err.Error())
	}
}

func TestClient_GetNextPage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/delta") || r.URL.Query().Get("skipToken") != "page2" {
			t.Errorf("Path = %q, want to contain /delta?skipToken=page2", r.URL.RequestURI())
		}

		deltaLink := "http://" + r.Host + "/v1.0/me/drive/root/delta?token=final"
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DeltaResponse{
			Value: []DriveItem{
				{ID: "item-2", Name: "file2.txt"},
			},
			DeltaLink: deltaLink,
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	var result DeltaResponse
	nextLink := ts.URL + "/v1.0/me/drive/root/delta?skipToken=page2"
	err := client.GetNextPage(context.Background(), nextLink, &result)
	if err != nil {
		t.Fatalf("GetNextPage() error = %v", err)
	}

	if len(result.Value) != 1 {
		t.Errorf("Value length = %d, want 1", len(result.Value))
	}
}

func TestClient_RateLimiting(t *testing.T) {
	requestTimes := []time.Time{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-123",
			DisplayName: "Test User",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   2.0, // 2 requests per second for faster test
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	// Make 3 requests
	for i := 0; i < 3; i++ {
		_, err := client.GetMe(context.Background())
		if err != nil {
			t.Fatalf("GetMe() error = %v", err)
		}
	}

	if len(requestTimes) != 3 {
		t.Fatalf("Expected 3 requests, got %d", len(requestTimes))
	}

	// Check that rate limiting was applied
	// With 2 RPS, the third request should be delayed by ~500ms from the second
	delay := requestTimes[2].Sub(requestTimes[1])
	minDelay := 400 * time.Millisecond
	if delay < minDelay {
		t.Errorf("Rate limiting not applied: delay = %v, want >= %v", delay, minDelay)
	}
}

func TestClient_RetryOnServerError(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Return 500 on first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error: &ErrorDetail{
					Code:    "internalServerError",
					Message: "Server error",
				},
			})
			return
		}

		// Success on third attempt
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-123",
			DisplayName: "Test User",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0, // High rate limit to speed up test
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v (should succeed after retries)", err)
	}

	if attemptCount != 3 {
		t.Errorf("Attempt count = %d, want 3 (2 retries + 1 success)", attemptCount)
	}
}

func TestClient_RetryOnRateLimit(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			// Return 429 on first attempt
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Error: &ErrorDetail{
					Code:    "activityLimitReached",
					Message: "Rate limit exceeded",
				},
			})
			return
		}

		// Success on second attempt
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-123",
			DisplayName: "Test User",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v (should succeed after retry)", err)
	}

	if attemptCount != 2 {
		t.Errorf("Attempt count = %d, want 2 (1 retry after 429)", attemptCount)
	}
}

func TestClient_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error: &ErrorDetail{
				Code:    "unauthenticated",
				Message: "Invalid token",
			},
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unauthenticated") {
		t.Errorf("error = %q, want to contain 'unauthenticated'", err.Error())
	}

	if !strings.Contains(err.Error(), "Invalid token") {
		t.Errorf("error = %q, want to contain 'Invalid token'", err.Error())
	}
}

func TestClient_DefaultConfig(t *testing.T) {
	cfg := DefaultClientConfig()

	if cfg.BaseURL != "https://graph.microsoft.com/v1.0" {
		t.Errorf("BaseURL = %q, want https://graph.microsoft.com/v1.0", cfg.BaseURL)
	}

	if cfg.RateLimitRPS != 10.0 {
		t.Errorf("RateLimitRPS = %f, want 10.0", cfg.RateLimitRPS)
	}

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s", cfg.RequestTimeout)
	}

	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
}

func TestDriveItem_IsFile(t *testing.T) {
	item := &DriveItem{
		File: &FileFacet{},
	}
	if !item.IsFile() {
		t.Error("IsFile() = false, want true")
	}

	item = &DriveItem{
		Folder: &FolderFacet{},
	}
	if item.IsFile() {
		t.Error("IsFile() = true, want false")
	}
}

func TestDriveItem_IsFolder(t *testing.T) {
	item := &DriveItem{
		Folder: &FolderFacet{},
	}
	if !item.IsFolder() {
		t.Error("IsFolder() = false, want true")
	}

	item = &DriveItem{
		File: &FileFacet{},
	}
	if item.IsFolder() {
		t.Error("IsFolder() = true, want false")
	}
}

func TestDriveItem_IsDeleted(t *testing.T) {
	item := &DriveItem{
		Deleted: &DeletedFacet{
			State: "deleted",
		},
	}
	if !item.IsDeleted() {
		t.Error("IsDeleted() = false, want true")
	}

	item = &DriveItem{
		File: &FileFacet{},
	}
	if item.IsDeleted() {
		t.Error("IsDeleted() = true, want false")
	}
}

// TestNewClient_NilTransportFallsBackToDefault verifies that when nil is
// passed for the transport parameter, the client falls back to
// http.DefaultTransport.
func TestNewClient_NilTransportFallsBackToDefault(t *testing.T) {
	tokenProvider := &stubTokenProvider{}
	cfg := &ClientConfig{
		BaseURL:        "https://graph.microsoft.com/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}

	// Create client with nil transport
	client := NewClient(tokenProvider, cfg, nil)

	if client.httpClient.Transport == nil {
		t.Error("expected httpClient.Transport to be non-nil")
	}

	// Should use http.DefaultTransport
	if client.httpClient.Transport != http.DefaultTransport {
		t.Error("expected httpClient.Transport to be http.DefaultTransport")
	}
}

// TestNewClient_UsesInjectedTransport verifies that when a custom
// http.RoundTripper is passed, the client uses it instead of the default.
func TestNewClient_UsesInjectedTransport(t *testing.T) {
	tokenProvider := &stubTokenProvider{}

	// Create a stub transport to track calls
	stubTransport := &stubRoundTripper{
		onRoundTrip: func(req *http.Request) (*http.Response, error) {
			// Return a minimal valid response
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
			}, nil
		},
	}

	cfg := &ClientConfig{
		BaseURL:        "https://graph.example.com/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}

	client := NewClient(tokenProvider, cfg, stubTransport)

	// Verify the transport is used
	if client.httpClient.Transport != stubTransport {
		t.Error("expected httpClient.Transport to be the injected transport")
	}

	// Make a request through the client to verify the transport is invoked
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-123",
			DisplayName: "Test User",
		})
	}))
	defer ts.Close()

	// Create a client with custom config pointing to test server
	testCfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}

	// Use a simple tracking transport
	callTracker := &callTrackingTransport{
		wrapped: http.DefaultTransport,
	}
	testClient := NewClient(tokenProvider, testCfg, callTracker)

	// Make a request
	_, err := testClient.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}

	// Verify the tracking transport was invoked
	if callTracker.callCount == 0 {
		t.Error("expected injected transport to be called")
	}
}

// stubRoundTripper is a test implementation of http.RoundTripper
type stubRoundTripper struct {
	onRoundTrip func(*http.Request) (*http.Response, error)
}

func (s *stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.onRoundTrip != nil {
		return s.onRoundTrip(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
	}, nil
}

// callTrackingTransport wraps an http.RoundTripper and tracks calls
type callTrackingTransport struct {
	wrapped   http.RoundTripper
	callCount int
}

func (c *callTrackingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.callCount++
	return c.wrapped.RoundTrip(req)
}

// TestGetDelta_ResyncRequired_410WithCode — Microsoft Graph signals
// "your stored delta token has aged out, restart the cycle" via HTTP
// 410 Gone with a resyncRequired-flavour error code. The client must
// surface this as the typed ErrResyncRequired sentinel so connector
// code can branch on it.
func TestGetDelta_ResyncRequired_410WithCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error: &ErrorDetail{
				Code:    "resyncRequired",
				Message: "Resync required.",
			},
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetDelta(context.Background(), "stale-token")
	if err == nil {
		t.Fatal("GetDelta expected error, got nil")
	}
	if !errors.Is(err, ErrResyncRequired) {
		t.Errorf("expected ErrResyncRequired, got %v", err)
	}
	// Underlying message should still be reachable for log context.
	if !strings.Contains(err.Error(), "resyncRequired") {
		t.Errorf("error should preserve upstream code, got: %v", err)
	}
}

// TestGetDelta_ResyncRequired_410NoBody — defensive: even if Graph
// returns 410 without a parseable error body, the client still maps
// it to ErrResyncRequired so callers get consistent recovery.
func TestGetDelta_ResyncRequired_410NoBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte("gone"))
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetDelta(context.Background(), "")
	if err == nil {
		t.Fatal("GetDelta expected error, got nil")
	}
	if !errors.Is(err, ErrResyncRequired) {
		t.Errorf("expected ErrResyncRequired even without parseable body, got %v", err)
	}
}

// TestGetDelta_NonResyncErrorPassesThrough — non-410 errors must NOT
// be conflated with resync. A 401 or 500 should stay as a generic
// error and let the caller's retry logic handle it.
func TestGetDelta_NonResyncErrorPassesThrough(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(ErrorResponse{
			Error: &ErrorDetail{Code: "Unauthorized", Message: "token expired"},
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   10.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetDelta(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrResyncRequired) {
		t.Errorf("401 should not map to ErrResyncRequired, got %v", err)
	}
}

// TestDoRequest_AllRetriesReturn429_SurfacesExhaustion verifies that when every
// attempt returns 429, the caller gets a clear "retries exhausted" error rather
// than the misleading "file already closed" that results from ReadAll on a body
// closed inside the retry loop.
func TestDoRequest_AllRetriesReturn429_SurfacesExhaustion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "retries exhausted") {
		t.Errorf("error = %q, want to contain 'retries exhausted'", msg)
	}
	if !strings.Contains(msg, "status 429") {
		t.Errorf("error = %q, want to contain 'status 429'", msg)
	}
	if !strings.Contains(msg, "after 3 attempts") {
		t.Errorf("error = %q, want to contain 'after 3 attempts'", msg)
	}
	if strings.Contains(msg, "file already closed") {
		t.Errorf("error = %q, must not contain 'file already closed'", msg)
	}
}

// TestDoRequest_AllRetriesReturn503_SurfacesExhaustion verifies the same
// exhaustion-error path for sustained 5xx responses.
func TestDoRequest_AllRetriesReturn503_SurfacesExhaustion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	_, err := client.GetMe(context.Background())
	if err == nil {
		t.Fatal("GetMe() expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "retries exhausted") {
		t.Errorf("error = %q, want to contain 'retries exhausted'", msg)
	}
	if !strings.Contains(msg, "status 503") {
		t.Errorf("error = %q, want to contain 'status 503'", msg)
	}
	if !strings.Contains(msg, "after 3 attempts") {
		t.Errorf("error = %q, want to contain 'after 3 attempts'", msg)
	}
	if strings.Contains(msg, "file already closed") {
		t.Errorf("error = %q, must not contain 'file already closed'", msg)
	}
}

// TestDoRequest_429ThenSuccess_ReturnsSuccess verifies that a single 429
// followed by a 200 succeeds and the response body is correctly unmarshalled.
func TestDoRequest_429ThenSuccess_ReturnsSuccess(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-abc",
			DisplayName: "Retry User",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v (should succeed after 429 retry)", err)
	}
	if user.ID != "user-abc" {
		t.Errorf("user.ID = %q, want user-abc", user.ID)
	}
	if attemptCount != 2 {
		t.Errorf("attemptCount = %d, want 2", attemptCount)
	}
}

// TestDoRequest_500ThenSuccess_ReturnsSuccess verifies that a single 500
// followed by a 200 succeeds and the response body is correctly unmarshalled.
func TestDoRequest_500ThenSuccess_ReturnsSuccess(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-xyz",
			DisplayName: "Retry User 500",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v (should succeed after 500 retry)", err)
	}
	if user.ID != "user-xyz" {
		t.Errorf("user.ID = %q, want user-xyz", user.ID)
	}
	if attemptCount != 2 {
		t.Errorf("attemptCount = %d, want 2", attemptCount)
	}
}

// TestDoRequest_ImmediateSuccess_NoRetries verifies that a 200 response on the
// first attempt succeeds with exactly one request.
func TestDoRequest_ImmediateSuccess_NoRetries(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(User{
			ID:          "user-immediate",
			DisplayName: "Immediate User",
		})
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	user, err := client.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe() error = %v", err)
	}
	if user.ID != "user-immediate" {
		t.Errorf("user.ID = %q, want user-immediate", user.ID)
	}
	if requestCount != 1 {
		t.Errorf("requestCount = %d, want 1 (no retries on immediate success)", requestCount)
	}
}

// TestWaitForRateLimit_ReturnsAfterTokenAvailable verifies that WaitForRateLimit
// returns without error when the per-client token bucket has capacity. The second
// call may block briefly (one token per 500ms at 2 RPS) but must still succeed.
func TestWaitForRateLimit_ReturnsAfterTokenAvailable(t *testing.T) {
	cfg := &ClientConfig{
		BaseURL:        "https://graph.microsoft.com/v1.0",
		RateLimitRPS:   2.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     0,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	ctx := context.Background()

	// First call: bucket starts full, returns immediately.
	if err := client.WaitForRateLimit(ctx); err != nil {
		t.Fatalf("WaitForRateLimit() first call error = %v", err)
	}

	// Second call: must also complete without error (may block up to ~500ms).
	if err := client.WaitForRateLimit(ctx); err != nil {
		t.Fatalf("WaitForRateLimit() second call error = %v", err)
	}
}

// TestWaitForRateLimit_RespectsCancelledContext verifies that WaitForRateLimit
// returns context.Canceled immediately when the supplied context is already
// cancelled, rather than blocking until a token becomes available.
func TestWaitForRateLimit_RespectsCancelledContext(t *testing.T) {
	// Near-zero refill rate so the bucket stays empty after the first drain.
	cfg := &ClientConfig{
		BaseURL:        "https://graph.microsoft.com/v1.0",
		RateLimitRPS:   0.001,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     0,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	// Drain the initial token using a live context.
	if err := client.WaitForRateLimit(context.Background()); err != nil {
		t.Fatalf("drain call error = %v", err)
	}

	// Pass an already-cancelled context — must not block.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.WaitForRateLimit(ctx)
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("WaitForRateLimit() with cancelled context = %v, want context.Canceled", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("WaitForRateLimit() did not return promptly on cancelled context")
	}
}

// TestParseRetryAfter checks the ParseRetryAfter helper against a range of
// header values covering the delta-seconds form (RFC 7231), whitespace
// tolerance, edge cases, and HTTP-date form (not supported; must return 0).
func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		header string
		want   time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{" 10 ", 10 * time.Second},
		{"-3", 0},
		{"abc", 0},
		{"Wed, 21 Oct 2015 07:28:00 GMT", 0},
	}
	for _, tc := range cases {
		got := ParseRetryAfter(tc.header)
		if got != tc.want {
			t.Errorf("ParseRetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
		}
	}
}

// TestDoRequest_HonorsRetryAfterOn429 verifies that when the server returns
// 429 with Retry-After: 2, the client waits at least 2 seconds before
// retrying and ultimately returns the subsequent 200 response.
func TestDoRequest_HonorsRetryAfterOn429(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	start := time.Now()
	var result struct{ OK bool `json:"ok"` }
	err := client.doRequest(context.Background(), "GET", "/me", nil, &result)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if !result.OK {
		t.Errorf("result.OK = false, want true")
	}
	if elapsed < 1900*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 1.9s (Retry-After: 2 not honoured)", elapsed)
	}
	if attemptCount != 2 {
		t.Errorf("attemptCount = %d, want 2", attemptCount)
	}
}

// TestDoRequest_HonorsRetryAfterOn503 verifies that the Retry-After header is
// honoured on 5xx responses, not only on 429.
func TestDoRequest_HonorsRetryAfterOn503(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	start := time.Now()
	var result struct{ OK bool `json:"ok"` }
	err := client.doRequest(context.Background(), "GET", "/me", nil, &result)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if !result.OK {
		t.Errorf("result.OK = false, want true")
	}
	if elapsed < 1900*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 1.9s (Retry-After: 2 not honoured on 503)", elapsed)
	}
	if attemptCount != 2 {
		t.Errorf("attemptCount = %d, want 2", attemptCount)
	}
}

// TestDoRequest_429BackoffFloorsAtOneSecond verifies that when a 429 response
// carries Retry-After: 0 (or a value that parses to zero), the backoff is
// floored at 1 second rather than hammering the server immediately.
// With Retry-After: 0, ParseRetryAfter returns 0, the fallback is
// (attempt+1)*1s = 1s at attempt 0, and the floor leaves it at 1s.
func TestDoRequest_429BackoffFloorsAtOneSecond(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	start := time.Now()
	var result struct{ OK bool `json:"ok"` }
	err := client.doRequest(context.Background(), "GET", "/me", nil, &result)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 0.9s (floor at 1s not applied for 429)", elapsed)
	}
	if attemptCount != 2 {
		t.Errorf("attemptCount = %d, want 2", attemptCount)
	}
}

// TestDoRequest_FallsBackToExponentialBackoffWhenNoRetryAfter verifies that
// when no Retry-After header is present on a 429, the client falls back to
// the (attempt+1)*1s exponential schedule. At attempt 0 the backoff is 1s.
func TestDoRequest_FallsBackToExponentialBackoffWhenNoRetryAfter(t *testing.T) {
	attemptCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cfg := &ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 5 * time.Second,
		MaxRetries:     2,
	}
	client := NewClient(&stubTokenProvider{}, cfg, nil)

	start := time.Now()
	var result struct{ OK bool `json:"ok"` }
	err := client.doRequest(context.Background(), "GET", "/me", nil, &result)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 0.9s (exponential backoff of 1s at attempt 0)", elapsed)
	}
	if attemptCount != 2 {
		t.Errorf("attemptCount = %d, want 2", attemptCount)
	}
}

// TestDefaultClientConfig_MaxRetriesIs5 is a sanity check that the default
// retry budget has been raised to 5.
func TestDefaultClientConfig_MaxRetriesIs5(t *testing.T) {
	cfg := DefaultClientConfig()
	if cfg.MaxRetries != 5 {
		t.Errorf("DefaultClientConfig().MaxRetries = %d, want 5", cfg.MaxRetries)
	}
}
