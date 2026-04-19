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
)

func TestContainerLister_ListContainers_Root(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		if r.URL.Path != "/v1.0/me/drive/items/root/children" {
			t.Errorf("Path = %q, want /v1.0/me/drive/items/root/children", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
			Value: []microsoft.DriveItem{
				{
					ID:                   "folder-1",
					Name:                 "Documents",
					WebURL:               "https://onedrive.com/Documents",
					CreatedDateTime:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					LastModifiedDateTime: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
					Folder: &microsoft.FolderFacet{
						ChildCount: 10,
					},
				},
				{
					ID:                   "folder-2",
					Name:                 "Photos",
					WebURL:               "https://onedrive.com/Photos",
					CreatedDateTime:      time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					LastModifiedDateTime: time.Date(2024, 1, 6, 0, 0, 0, 0, time.UTC),
					Folder: &microsoft.FolderFacet{
						ChildCount: 0,
					},
				},
				{
					ID:   "file-1",
					Name: "readme.txt",
					File: &microsoft.FileFacet{
						MimeType: "text/plain",
					},
				},
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	containers, cursor, err := lister.ListContainers(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}

	// Should return only folders, not files
	if len(containers) != 2 {
		t.Fatalf("Containers length = %d, want 2 (only folders)", len(containers))
	}

	// Check first container
	c1 := containers[0]
	if c1.ID != "folder-1:Documents" {
		t.Errorf("Container[0].ID = %q, want folder-1:Documents", c1.ID)
	}

	if c1.Name != "Documents" {
		t.Errorf("Container[0].Name = %q, want Documents", c1.Name)
	}

	if c1.Type != "folder" {
		t.Errorf("Container[0].Type = %q, want folder", c1.Type)
	}

	if !c1.HasChildren {
		t.Error("Container[0].HasChildren = false, want true (childCount > 0)")
	}

	if c1.Description != "Folder with 10 items" {
		t.Errorf("Container[0].Description = %q, want 'Folder with 10 items'", c1.Description)
	}

	// Check metadata
	if c1.Metadata["web_url"] != "https://onedrive.com/Documents" {
		t.Errorf("Container[0].Metadata['web_url'] = %q", c1.Metadata["web_url"])
	}

	if c1.Metadata["child_count"] != "10" {
		t.Errorf("Container[0].Metadata['child_count'] = %q, want 10", c1.Metadata["child_count"])
	}

	// Check second container (empty folder)
	c2 := containers[1]
	if c2.HasChildren {
		t.Error("Container[1].HasChildren = true, want false (childCount = 0)")
	}

	if cursor != "" {
		t.Errorf("Cursor = %q, want empty (no pagination)", cursor)
	}
}

func TestContainerLister_ListContainers_WithParentID(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Method = %q, want GET", r.Method)
		}

		// Should request children of folder-123
		if r.URL.Path != "/v1.0/me/drive/items/folder-123/children" {
			t.Errorf("Path = %q, want /v1.0/me/drive/items/folder-123/children", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
			Value: []microsoft.DriveItem{
				{
					ID:   "subfolder-1",
					Name: "2024",
					Folder: &microsoft.FolderFacet{
						ChildCount: 5,
					},
				},
				{
					ID:   "subfolder-2",
					Name: "Archive",
					Folder: &microsoft.FolderFacet{
						ChildCount: 0,
					},
				},
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	// Use parentID format "id:name" - should extract the ID part
	containers, _, err := lister.ListContainers(context.Background(), "", "folder-123:Documents")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}

	if len(containers) != 2 {
		t.Fatalf("Containers length = %d, want 2", len(containers))
	}

	if containers[0].ID != "subfolder-1:2024" {
		t.Errorf("Container[0].ID = %q, want subfolder-1:2024", containers[0].ID)
	}

	if containers[0].Name != "2024" {
		t.Errorf("Container[0].Name = %q, want 2024", containers[0].Name)
	}
}

func TestContainerLister_ListContainers_ParentIDParsing(t *testing.T) {
	tests := []struct {
		name         string
		parentID     string
		expectedPath string
	}{
		{
			name:         "empty parent",
			parentID:     "",
			expectedPath: "/v1.0/me/drive/items/root/children",
		},
		{
			name:         "id only",
			parentID:     "folder-123",
			expectedPath: "/v1.0/me/drive/items/folder-123/children",
		},
		{
			name:         "id with name",
			parentID:     "folder-123:Documents",
			expectedPath: "/v1.0/me/drive/items/folder-123/children",
		},
		{
			name:         "name with colon",
			parentID:     "folder-456:Project:2024",
			expectedPath: "/v1.0/me/drive/items/folder-456/children",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathChecked := false
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tt.expectedPath {
					pathChecked = true
				}

				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
					Value: []microsoft.DriveItem{},
				})
			}))
			defer ts.Close()

			config := &Config{
				RateLimitRPS:   100.0,
				RequestTimeout: 30 * time.Second,
				MaxRetries:     3,
				MaxFileSize:    100 * 1024 * 1024,
			}

			lister := NewContainerLister(&stubTokenProvider{}, config)
			lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
				BaseURL:        ts.URL + "/v1.0",
				RateLimitRPS:   100.0,
				RequestTimeout: 30 * time.Second,
				MaxRetries:     3,
			})

			_, _, err := lister.ListContainers(context.Background(), "", tt.parentID)
			if err != nil {
				t.Fatalf("ListContainers() error = %v", err)
			}

			if !pathChecked {
				t.Errorf("Expected path %q was not used", tt.expectedPath)
			}
		})
	}
}

func TestContainerLister_ListContainers_Pagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "skipToken=page2") {
			// Second page
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
				Value: []microsoft.DriveItem{
					{
						ID:     "folder-2",
						Name:   "Folder2",
						Folder: &microsoft.FolderFacet{ChildCount: 2},
					},
				},
			})
		} else if r.URL.Path == "/v1.0/me/drive/items/root/children" {
			// First page
			nextLink := "http://" + r.Host + "/v1.0/me/drive/items/root/children?skipToken=page2"
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
				Value: []microsoft.DriveItem{
					{
						ID:     "folder-1",
						Name:   "Folder1",
						Folder: &microsoft.FolderFacet{ChildCount: 1},
					},
				},
				NextLink: nextLink,
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	// First page
	containers, cursor, err := lister.ListContainers(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListContainers() page 1 error = %v", err)
	}

	if len(containers) != 1 {
		t.Errorf("Page 1 containers length = %d, want 1", len(containers))
	}

	if containers[0].Name != "Folder1" {
		t.Errorf("Page 1 container name = %q, want Folder1", containers[0].Name)
	}

	if cursor == "" {
		t.Fatal("Cursor is empty, want nextLink for pagination")
	}

	if !strings.Contains(cursor, "skipToken=page2") {
		t.Errorf("Cursor = %q, want to contain skipToken=page2", cursor)
	}

	// Second page
	containers, cursor, err = lister.ListContainers(context.Background(), cursor, "")
	if err != nil {
		t.Fatalf("ListContainers() page 2 error = %v", err)
	}

	if len(containers) != 1 {
		t.Errorf("Page 2 containers length = %d, want 1", len(containers))
	}

	if containers[0].Name != "Folder2" {
		t.Errorf("Page 2 container name = %q, want Folder2", containers[0].Name)
	}

	if cursor != "" {
		t.Errorf("Cursor = %q, want empty (no more pages)", cursor)
	}
}

func TestContainerLister_ListContainers_OnlyFolders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
			Value: []microsoft.DriveItem{
				{
					ID:     "folder-1",
					Name:   "ValidFolder",
					Folder: &microsoft.FolderFacet{ChildCount: 3},
				},
				{
					ID:   "file-1",
					Name: "document.pdf",
					File: &microsoft.FileFacet{MimeType: "application/pdf"},
				},
				{
					ID:   "file-2",
					Name: "image.jpg",
					File: &microsoft.FileFacet{MimeType: "image/jpeg"},
				},
				{
					ID:     "folder-2",
					Name:   "AnotherFolder",
					Folder: &microsoft.FolderFacet{ChildCount: 0},
				},
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	containers, _, err := lister.ListContainers(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}

	// Should only return folders, not files
	if len(containers) != 2 {
		t.Errorf("Containers length = %d, want 2 (files should be excluded)", len(containers))
	}

	for _, c := range containers {
		if c.Type != "folder" {
			t.Errorf("Container type = %q, want folder", c.Type)
		}
	}
}

func TestContainerLister_ListContainers_Error(t *testing.T) {
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	_, _, err := lister.ListContainers(context.Background(), "", "")
	if err == nil {
		t.Fatal("ListContainers() expected error, got nil")
	}

	if !strings.Contains(err.Error(), "list drive items") {
		t.Errorf("error = %q, want to contain 'list drive items'", err.Error())
	}
}

func TestContainerLister_ContainerIDFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
			Value: []microsoft.DriveItem{
				{
					ID:     "abc123",
					Name:   "My Documents",
					Folder: &microsoft.FolderFacet{ChildCount: 5},
				},
				{
					ID:     "def456",
					Name:   "Folder:With:Colons",
					Folder: &microsoft.FolderFacet{ChildCount: 0},
				},
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	containers, _, err := lister.ListContainers(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}

	// Check format is "id:name"
	if containers[0].ID != "abc123:My Documents" {
		t.Errorf("Container[0].ID = %q, want abc123:My Documents", containers[0].ID)
	}

	// Names with colons should be preserved
	if containers[1].ID != "def456:Folder:With:Colons" {
		t.Errorf("Container[1].ID = %q, want def456:Folder:With:Colons", containers[1].ID)
	}

	if containers[1].Name != "Folder:With:Colons" {
		t.Errorf("Container[1].Name = %q, want Folder:With:Colons", containers[1].Name)
	}
}

func TestContainerLister_MetadataFormatting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(microsoft.DriveItemsResponse{
			Value: []microsoft.DriveItem{
				{
					ID:                   "folder-1",
					Name:                 "Test Folder",
					WebURL:               "https://onedrive.com/folders/test",
					CreatedDateTime:      time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
					LastModifiedDateTime: time.Date(2024, 3, 20, 14, 45, 0, 0, time.UTC),
					Folder: &microsoft.FolderFacet{
						ChildCount: 42,
					},
				},
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

	lister := NewContainerLister(&stubTokenProvider{}, config)
	lister.client = microsoft.NewClient(&stubTokenProvider{}, &microsoft.ClientConfig{
		BaseURL:        ts.URL + "/v1.0",
		RateLimitRPS:   100.0,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
	})

	containers, _, err := lister.ListContainers(context.Background(), "", "")
	if err != nil {
		t.Fatalf("ListContainers() error = %v", err)
	}

	c := containers[0]

	// Check metadata values
	if c.Metadata["web_url"] != "https://onedrive.com/folders/test" {
		t.Errorf("Metadata['web_url'] = %q", c.Metadata["web_url"])
	}

	if c.Metadata["created_datetime"] != "2024-03-15" {
		t.Errorf("Metadata['created_datetime'] = %q, want 2024-03-15", c.Metadata["created_datetime"])
	}

	if c.Metadata["modified_datetime"] != "2024-03-20" {
		t.Errorf("Metadata['modified_datetime'] = %q, want 2024-03-20", c.Metadata["modified_datetime"])
	}

	if c.Metadata["child_count"] != "42" {
		t.Errorf("Metadata['child_count'] = %q, want 42", c.Metadata["child_count"])
	}

	if c.Description != "Folder with 42 items" {
		t.Errorf("Description = %q, want 'Folder with 42 items'", c.Description)
	}
}
