package microsoft

import (
	"context"
	"encoding/json"
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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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
	client := NewClient(&stubTokenProvider{}, cfg)

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

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
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
