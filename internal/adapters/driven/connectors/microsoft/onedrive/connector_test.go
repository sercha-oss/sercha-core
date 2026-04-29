package onedrive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sercha-oss/sercha-core/internal/adapters/driven/connectors/microsoft"
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

func TestConnector_Type(t *testing.T) {
	connector := NewConnector(&stubTokenProvider{}, "", nil)

	if connector.Type() != domain.ProviderTypeOneDrive {
		t.Errorf("Type() = %v, want %v", connector.Type(), domain.ProviderTypeOneDrive)
	}
}

func TestConnector_ValidateConfig(t *testing.T) {
	connector := NewConnector(&stubTokenProvider{}, "", nil)

	// OneDrive has no special config validation
	err := connector.ValidateConfig(domain.SourceConfig{})
	if err != nil {
		t.Errorf("ValidateConfig() error = %v, want nil", err)
	}
}

func TestConnector_FetchChanges_InitialSync(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/me/drive/root/delta" {
			t.Errorf("Path = %q, want /v1.0/me/drive/root/delta", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
			Value: []microsoft.DriveItem{
				{
					ID:                   "file-123",
					Name:                 "document.txt",
					Size:                 1024,
					WebURL:               "https://onedrive.com/document.txt",
					CreatedDateTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					LastModifiedDateTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					File: &microsoft.FileFacet{
						MimeType: "text/plain",
					},
					ParentReference: &microsoft.ItemReference{
						ID:   "root",
						Path: "/drive/root:",
					},
				},
			},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=abc123",
		})
	}))
	defer ts.Close()

	// Create connector with test server
	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	// Override client to use test server
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{
		ID:   "source-1",
		Name: "Test Source",
	}

	// Mock file content download
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/content") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hello, OneDrive!"))
		}
	}))
	defer ts2.Close()

	// Replace base URL for content downloads
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts2.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	// Override GetDelta to use first test server
	// We'll need to create a mock that handles both delta and content
	combinedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/delta") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
				Value: []microsoft.DriveItem{
					{
						ID:                   "file-123",
						Name:                 "document.txt",
						Size:                 1024,
						WebURL:               "https://onedrive.com/document.txt",
						CreatedDateTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
						LastModifiedDateTime: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
						File: &microsoft.FileFacet{
							MimeType: "text/plain",
						},
						ParentReference: &microsoft.ItemReference{
							ID:   "root",
							Path: "/drive/root:",
						},
					},
				},
				DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=abc123",
			})
		} else if strings.Contains(r.URL.Path, "/content") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hello, OneDrive!"))
		}
	}))
	defer combinedServer.Close()

	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        combinedServer.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	changes, cursor, err := connector.FetchChanges(context.Background(), source, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Changes length = %d, want 1", len(changes))
	}

	change := changes[0]
	if change.Type != domain.ChangeTypeModified {
		t.Errorf("Change type = %v, want %v", change.Type, domain.ChangeTypeModified)
	}

	if change.ExternalID != "file-file-123" {
		t.Errorf("ExternalID = %q, want file-file-123", change.ExternalID)
	}

	if change.Document.Title != "document.txt" {
		t.Errorf("Document title = %q, want document.txt", change.Document.Title)
	}

	if change.LoadContent == nil {
		t.Fatal("expected LoadContent thunk to be set")
	}
	loaded, err := change.LoadContent(context.Background())
	if err != nil {
		t.Fatalf("LoadContent error = %v", err)
	}
	if loaded != "Hello, OneDrive!" {
		t.Errorf("Content = %q, want 'Hello, OneDrive!'", loaded)
	}

	if cursor == "" {
		t.Error("Cursor is empty, want delta link")
	}
}

func TestConnector_FetchChanges_DeletedFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
			Value: []microsoft.DriveItem{
				{
					ID:   "file-deleted",
					Name: "deleted.txt",
					Deleted: &microsoft.DeletedFacet{
						State: "deleted",
					},
				},
			},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=xyz",
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	changes, _, err := connector.FetchChanges(context.Background(), source, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Changes length = %d, want 1", len(changes))
	}

	if changes[0].Type != domain.ChangeTypeDeleted {
		t.Errorf("Change type = %v, want %v", changes[0].Type, domain.ChangeTypeDeleted)
	}

	if changes[0].ExternalID != "file-file-deleted" {
		t.Errorf("ExternalID = %q, want file-file-deleted", changes[0].ExternalID)
	}
}

func TestConnector_FetchChanges_SkipFolders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
			Value: []microsoft.DriveItem{
				{
					ID:   "folder-123",
					Name: "Documents",
					Folder: &microsoft.FolderFacet{
						ChildCount: 10,
					},
				},
			},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=xyz",
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	changes, _, err := connector.FetchChanges(context.Background(), source, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	// Folders should be skipped
	if len(changes) != 0 {
		t.Errorf("Changes length = %d, want 0 (folders should be skipped)", len(changes))
	}
}

func TestConnector_FetchChanges_SkipLargeFiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
			Value: []microsoft.DriveItem{
				{
					ID:   "large-file",
					Name: "huge.zip",
					Size: 200 * 1024 * 1024, // 200 MB - exceeds default max
					File: &microsoft.FileFacet{
						MimeType: "application/zip",
					},
				},
			},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=xyz",
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024, // 100 MB max
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	changes, _, err := connector.FetchChanges(context.Background(), source, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	// Large files should be skipped
	if len(changes) != 0 {
		t.Errorf("Changes length = %d, want 0 (large files should be skipped)", len(changes))
	}
}

func TestConnector_FetchChanges_WithContainer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
			Value: []microsoft.DriveItem{
				{
					ID:   "file-in-folder",
					Name: "doc.txt",
					Size: 512,
					File: &microsoft.FileFacet{MimeType: "text/plain"},
					ParentReference: &microsoft.ItemReference{
						ID:   "folder-123",
						Path: "/drive/root:/MyFolder",
					},
				},
				{
					ID:   "file-outside",
					Name: "other.txt",
					Size: 512,
					File: &microsoft.FileFacet{MimeType: "text/plain"},
					ParentReference: &microsoft.ItemReference{
						ID:   "root",
						Path: "/drive/root:",
					},
				},
			},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=xyz",
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	// Container format: "folder-123:MyFolder"
	connector := NewConnector(&stubTokenProvider{}, "folder-123:MyFolder", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	// Mock content endpoint
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/delta") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
				Value: []microsoft.DriveItem{
					{
						ID:   "file-in-folder",
						Name: "doc.txt",
						Size: 512,
						File: &microsoft.FileFacet{MimeType: "text/plain"},
						ParentReference: &microsoft.ItemReference{
							ID:   "folder-123",
							Path: "/drive/root:/MyFolder",
						},
					},
					{
						ID:   "file-outside",
						Name: "other.txt",
						Size: 512,
						File: &microsoft.FileFacet{MimeType: "text/plain"},
						ParentReference: &microsoft.ItemReference{
							ID:   "root",
							Path: "/drive/root:",
						},
					},
				},
				DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=xyz",
			})
		} else if strings.Contains(r.URL.Path, "/content") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test content"))
		}
	}))
	defer ts2.Close()

	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts2.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	changes, _, err := connector.FetchChanges(context.Background(), source, "")
	if err != nil {
		t.Fatalf("FetchChanges() error = %v", err)
	}

	// Only file in the specified folder should be included
	if len(changes) != 1 {
		t.Fatalf("Changes length = %d, want 1 (only files in container)", len(changes))
	}

	if changes[0].ExternalID != "file-file-in-folder" {
		t.Errorf("ExternalID = %q, want file-file-in-folder", changes[0].ExternalID)
	}
}

func TestConnector_FetchDocument(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/items/file-123/content") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Document content"))
		} else if strings.Contains(r.URL.Path, "/items/file-123") {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(microsoft.DriveItem{
				ID:                   "file-123",
				Name:                 "report.pdf",
				Size:                 2048,
				WebURL:               "https://onedrive.com/report.pdf",
				CreatedDateTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastModifiedDateTime: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
				File: &microsoft.FileFacet{
					MimeType: "application/pdf",
				},
			})
		}
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	doc, contentHash, err := connector.FetchDocument(context.Background(), source, "file-file-123")
	if err != nil {
		t.Fatalf("FetchDocument() error = %v", err)
	}

	if doc.Title != "report.pdf" {
		t.Errorf("Title = %q, want report.pdf", doc.Title)
	}

	if doc.MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want application/pdf", doc.MimeType)
	}

	if contentHash == "" {
		t.Error("ContentHash is empty, want non-empty hash")
	}
}

func TestConnector_FetchDocument_InvalidExternalID(t *testing.T) {
	connector := NewConnector(&stubTokenProvider{}, "", nil)
	source := &domain.Source{ID: "source-1"}

	_, _, err := connector.FetchDocument(context.Background(), source, "invalid-id")
	if err == nil {
		t.Fatal("FetchDocument() expected error for invalid external ID, got nil")
	}

	if !strings.Contains(err.Error(), "invalid external ID format") {
		t.Errorf("error = %q, want to contain 'invalid external ID format'", err.Error())
	}
}

func TestConnector_FetchDocument_NotAFile(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DriveItem{
			ID:   "folder-123",
			Name: "Documents",
			Folder: &microsoft.FolderFacet{
				ChildCount: 5,
			},
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	_, _, err := connector.FetchDocument(context.Background(), source, "file-folder-123")
	if err == nil {
		t.Fatal("FetchDocument() expected error for folder, got nil")
	}

	if !strings.Contains(err.Error(), "not a file") {
		t.Errorf("error = %q, want to contain 'not a file'", err.Error())
	}
}

func TestConnector_TestConnection(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1.0/me" {
			t.Errorf("Path = %q, want /v1.0/me", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.User{
			ID:                "user-123",
			DisplayName:       "Test User",
			UserPrincipalName: "test@example.com",
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	err := connector.TestConnection(context.Background(), source)
	if err != nil {
		t.Errorf("TestConnection() error = %v, want nil", err)
	}
}

func TestConnector_TestConnection_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(microsoft.ErrorResponse{
			Error: &microsoft.ErrorDetail{
				Code:    "unauthenticated",
				Message: "Invalid token",
			},
		})
	}))
	defer ts.Close()

	config := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}

	connector := NewConnector(&stubTokenProvider{}, "", config)
	connector.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	source := &domain.Source{ID: "source-1"}

	err := connector.TestConnection(context.Background(), source)
	if err == nil {
		t.Fatal("TestConnection() expected error, got nil")
	}
}

func TestConnector_ContainerIDParsing(t *testing.T) {
	tests := []struct {
		name         string
		containerID  string
		expectedID   string
		expectedName string
	}{
		{
			name:         "empty container",
			containerID:  "",
			expectedID:   "",
			expectedName: "",
		},
		{
			name:         "id only",
			containerID:  "folder-123",
			expectedID:   "folder-123",
			expectedName: "",
		},
		{
			name:         "id and name",
			containerID:  "folder-123:Documents",
			expectedID:   "folder-123",
			expectedName: "Documents",
		},
		{
			name:         "name with colon",
			containerID:  "folder-123:Project:2024",
			expectedID:   "folder-123",
			expectedName: "Project:2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector := NewConnector(&stubTokenProvider{}, tt.containerID, nil)

			if connector.containerID != tt.expectedID {
				t.Errorf("containerID = %q, want %q", connector.containerID, tt.expectedID)
			}

			if connector.containerName != tt.expectedName {
				t.Errorf("containerName = %q, want %q", connector.containerName, tt.expectedName)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.RateLimitRPS != 10.0 {
		t.Errorf("RateLimitRPS = %f, want 10.0", cfg.RateLimitRPS)
	}

	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s", cfg.RequestTimeout)
	}

	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}

	if cfg.MaxFileSize != 100*1024*1024 {
		t.Errorf("MaxFileSize = %d, want 104857600 (100 MB)", cfg.MaxFileSize)
	}
}

// TestFetchChanges_DrainsMultipleDeltaPages — Microsoft Graph paginates
// large delta responses with @odata.nextLink. Items are not re-emitted
// on subsequent ticks, so FetchChanges MUST drain every nextLink in
// one call before returning the final deltaLink as the cursor.
func TestFetchChanges_DrainsMultipleDeltaPages(t *testing.T) {
	deltaCalls := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/content") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("body"))
			return
		}
		if !strings.Contains(r.URL.Path, "/delta") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		deltaCalls++
		w.WriteHeader(http.StatusOK)
		switch deltaCalls {
		case 1:
			_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
				Value: []microsoft.DriveItem{{
					ID: "f1", Name: "a.txt", Size: 10,
					File:            &microsoft.FileFacet{MimeType: "text/plain"},
					ParentReference: &microsoft.ItemReference{ID: "root", Path: "/drive/root:"},
				}},
				NextLink: server.URL + "/v1.0/me/drive/root/delta?token=page2",
			})
		case 2:
			_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
				Value: []microsoft.DriveItem{{
					ID: "f2", Name: "b.txt", Size: 10,
					File:            &microsoft.FileFacet{MimeType: "text/plain"},
					ParentReference: &microsoft.ItemReference{ID: "root", Path: "/drive/root:"},
				}},
				NextLink: server.URL + "/v1.0/me/drive/root/delta?token=page3",
			})
		case 3:
			_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
				Value: []microsoft.DriveItem{{
					ID: "f3", Name: "c.txt", Size: 10,
					File:            &microsoft.FileFacet{MimeType: "text/plain"},
					ParentReference: &microsoft.ItemReference{ID: "root", Path: "/drive/root:"},
				}},
				DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=final",
			})
		}
	}))
	defer server.Close()

	cfg := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}
	c := NewConnector(&stubTokenProvider{}, "", cfg)
	c.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        server.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	changes, cursor, err := c.FetchChanges(context.Background(), &domain.Source{ID: "src"}, "")
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}
	if deltaCalls != 3 {
		t.Errorf("expected 3 delta page fetches, got %d", deltaCalls)
	}
	if len(changes) != 3 {
		t.Errorf("expected 3 changes (one per page), got %d", len(changes))
	}
	want := map[string]bool{"file-f1": true, "file-f2": true, "file-f3": true}
	for _, ch := range changes {
		if !want[ch.ExternalID] {
			t.Errorf("unexpected change %q", ch.ExternalID)
		}
	}
	if !strings.Contains(cursor, "token=final") {
		t.Errorf("cursor should be the final deltaLink, got %q", cursor)
	}
}

// TestFetchChanges_RecoversFromResyncRequired — when the stored
// delta token has aged out, Graph returns 410 with resyncRequired.
// FetchChanges must catch this, drop any in-flight changes from the
// stale cycle, and restart with an empty cursor inside the same call
// so the orchestrator gets a usable result this tick.
func TestFetchChanges_RecoversFromResyncRequired(t *testing.T) {
	deltaCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/content") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("body"))
			return
		}
		if !strings.Contains(r.URL.Path, "/delta") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		deltaCalls++
		// First call: token is invalid → 410. The client maps this to
		// ErrResyncRequired; the connector's recovery branch resets
		// the cursor and calls again.
		if deltaCalls == 1 {
			w.WriteHeader(http.StatusGone)
			_ = json.NewEncoder(w).Encode(microsoft.ErrorResponse{
				Error: &microsoft.ErrorDetail{Code: "resyncRequired", Message: "token aged"},
			})
			return
		}
		// Second call: empty token → fresh cycle. Return a single file
		// and a final DeltaLink so the loop terminates cleanly.
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{
			Value: []microsoft.DriveItem{{
				ID: "fresh-1", Name: "x.txt", Size: 5,
				File:            &microsoft.FileFacet{MimeType: "text/plain"},
				ParentReference: &microsoft.ItemReference{ID: "root", Path: "/drive/root:"},
			}},
			DeltaLink: "https://graph.microsoft.com/v1.0/me/drive/root/delta?token=fresh",
		})
	}))
	defer ts.Close()

	cfg := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}
	c := NewConnector(&stubTokenProvider{}, "", cfg)
	c.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	// Pass a "stale" cursor that simulates a token Graph will reject.
	// The cursor needs to be a URL the client will hit, since GetDelta
	// trims the base URL and uses what's left as the path. Use the
	// same server's delta path so the 410 lands here.
	staleCursor := ts.URL + "/v1.0/me/drive/root/delta?token=stale"
	changes, cursor, err := c.FetchChanges(context.Background(), &domain.Source{ID: "src"}, staleCursor)
	if err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}
	if deltaCalls != 2 {
		t.Errorf("expected 2 delta calls (stale 410 + fresh), got %d", deltaCalls)
	}
	if len(changes) != 1 || changes[0].ExternalID != "file-fresh-1" {
		var got string
		for _, ch := range changes {
			got += ch.ExternalID + " "
		}
		t.Errorf("expected 1 change file-fresh-1, got %d (%s)", len(changes), got)
	}
	if !strings.Contains(cursor, "token=fresh") {
		t.Errorf("cursor should be fresh DeltaLink, got %q", cursor)
	}
}

// TestFetchChanges_DoesNotRetryRepeatedResyncRequired — defensive: if
// Graph 410s on the SECOND call too (after we already reset to empty),
// we must not loop forever. The bound-recovery flag guarantees one
// resync attempt per FetchChanges call.
func TestFetchChanges_DoesNotRetryRepeatedResyncRequired(t *testing.T) {
	deltaCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/delta") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		deltaCalls++
		w.WriteHeader(http.StatusGone)
		_ = json.NewEncoder(w).Encode(microsoft.ErrorResponse{
			Error: &microsoft.ErrorDetail{Code: "resyncRequired", Message: "still gone"},
		})
	}))
	defer ts.Close()

	cfg := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}
	c := NewConnector(&stubTokenProvider{}, "", cfg)
	c.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	_, _, err := c.FetchChanges(context.Background(), &domain.Source{ID: "src"}, "")
	if err == nil {
		t.Fatalf("expected error after repeated resyncRequired, got nil")
	}
	if deltaCalls != 2 {
		t.Errorf("expected exactly 2 calls (one initial + one recovery), got %d", deltaCalls)
	}
}

// TestFetchChanges_StopsWhenNeitherLinkPresent — defensive: if Graph
// ever returns a response with no DeltaLink AND no NextLink (shouldn't
// happen in practice but the schema technically allows), the loop
// terminates rather than spinning. Cursor is left unchanged so the
// next tick retries the same starting point.
func TestFetchChanges_StopsWhenNeitherLinkPresent(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/delta") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits++
		w.WriteHeader(http.StatusOK)
		// No DeltaLink, no NextLink — loop must break.
		_ = json.NewEncoder(w).Encode(microsoft.DeltaResponse{Value: nil})
	}))
	defer ts.Close()

	cfg := &Config{
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		MaxFileSize:    100 * 1024 * 1024,
	}
	c := NewConnector(&stubTokenProvider{}, "", cfg)
	c.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	if _, _, err := c.FetchChanges(context.Background(), &domain.Source{ID: "src"}, ""); err != nil {
		t.Fatalf("FetchChanges: %v", err)
	}
	if hits != 1 {
		t.Errorf("expected exactly 1 delta call when no continuation links, got %d", hits)
	}
}
