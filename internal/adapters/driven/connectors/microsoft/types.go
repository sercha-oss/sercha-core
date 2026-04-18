package microsoft

import "time"

// User represents a Microsoft Graph user.
type User struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	UserPrincipalName string `json:"userPrincipalName"` // Email
	Mail              string `json:"mail,omitempty"`
}

// DriveItem represents a file or folder in OneDrive.
type DriveItem struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Size                 int64          `json:"size,omitempty"`
	WebURL               string         `json:"webUrl"`
	CreatedDateTime      time.Time      `json:"createdDateTime"`
	LastModifiedDateTime time.Time      `json:"lastModifiedDateTime"`
	Deleted              *DeletedFacet  `json:"deleted,omitempty"`
	File                 *FileFacet     `json:"file,omitempty"`
	Folder               *FolderFacet   `json:"folder,omitempty"`
	ParentReference      *ItemReference `json:"parentReference,omitempty"`
	CTag                 string         `json:"cTag,omitempty"` // Change tag
	ETag                 string         `json:"eTag,omitempty"` // Entity tag
	DownloadURL          string         `json:"@microsoft.graph.downloadUrl,omitempty"`
}

// DeletedFacet indicates an item has been deleted.
type DeletedFacet struct {
	State string `json:"state,omitempty"`
}

// FileFacet represents file-specific metadata.
type FileFacet struct {
	MimeType string      `json:"mimeType,omitempty"`
	Hashes   *HashesType `json:"hashes,omitempty"`
}

// HashesType contains file hashes.
type HashesType struct {
	QuickXorHash string `json:"quickXorHash,omitempty"`
	SHA1Hash     string `json:"sha1Hash,omitempty"`
	SHA256Hash   string `json:"sha256Hash,omitempty"`
}

// FolderFacet represents folder-specific metadata.
type FolderFacet struct {
	ChildCount int `json:"childCount,omitempty"`
}

// ItemReference represents a reference to a parent item.
type ItemReference struct {
	DriveID   string `json:"driveId,omitempty"`
	DriveType string `json:"driveType,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
}

// DeltaResponse represents a response from a delta query.
type DeltaResponse struct {
	Value     []DriveItem `json:"value"`
	NextLink  string      `json:"@odata.nextLink,omitempty"`
	DeltaLink string      `json:"@odata.deltaLink,omitempty"`
}

// DriveItemsResponse represents a response when listing drive items.
type DriveItemsResponse struct {
	Value    []DriveItem `json:"value"`
	NextLink string      `json:"@odata.nextLink,omitempty"`
}

// ErrorResponse represents a Microsoft Graph API error.
type ErrorResponse struct {
	Error *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail contains error details.
type ErrorDetail struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	InnerError *struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"innerError,omitempty"`
}

// IsFile returns true if the drive item is a file.
func (d *DriveItem) IsFile() bool {
	return d.File != nil
}

// IsFolder returns true if the drive item is a folder.
func (d *DriveItem) IsFolder() bool {
	return d.Folder != nil
}

// IsDeleted returns true if the drive item has been deleted.
func (d *DriveItem) IsDeleted() bool {
	return d.Deleted != nil
}
