package localfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/domain"
	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// Ensure ContainerLister implements the interface.
var _ driven.ContainerLister = (*ContainerLister)(nil)

// ContainerLister lists subdirectories as containers.
type ContainerLister struct {
	basePath string
}

// NewContainerLister creates a ContainerLister for a base path.
func NewContainerLister(basePath string) *ContainerLister {
	return &ContainerLister{basePath: basePath}
}

// ListContainers lists immediate subdirectories as containers.
// Cursor is not used (all dirs returned at once for simplicity).
func (l *ContainerLister) ListContainers(ctx context.Context, cursor string, _ string) ([]*driven.Container, string, error) {
	// parentID ignored - localfs lists from basePath only
	entries, err := os.ReadDir(l.basePath)
	if err != nil {
		return nil, "", fmt.Errorf("read directory: %w", err)
	}

	var containers []*driven.Container
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		default:
		}

		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Count files in directory for description
		fileCount := countFiles(filepath.Join(l.basePath, entry.Name()))

		containers = append(containers, &driven.Container{
			ID:          entry.Name(),
			Name:        entry.Name(),
			Description: fmt.Sprintf("%d files", fileCount),
			Type:        "directory",
			Metadata: map[string]string{
				"full_path":  filepath.Join(l.basePath, entry.Name()),
				"mod_time":   info.ModTime().Format("2006-01-02 15:04:05"),
				"file_count": fmt.Sprintf("%d", fileCount),
			},
		})
	}

	return containers, "", nil
}

// countFiles counts the number of files in a directory (non-recursive).
func countFiles(path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			count++
		}
	}
	return count
}

// ContainerListerFactory creates ContainerListers for LocalFS installations.
type ContainerListerFactory struct {
	installationStore driven.ConnectionStore
}

// NewContainerListerFactory creates a factory for LocalFS container listers.
func NewContainerListerFactory(installationStore driven.ConnectionStore) *ContainerListerFactory {
	return &ContainerListerFactory{
		installationStore: installationStore,
	}
}

// Type returns the provider type this factory handles.
func (f *ContainerListerFactory) Type() domain.ProviderType {
	return domain.ProviderTypeLocalFS
}

// Create creates a ContainerLister for a LocalFS installation.
func (f *ContainerListerFactory) Create(ctx context.Context, installationID string) (driven.ContainerLister, error) {
	inst, err := f.installationStore.Get(ctx, installationID)
	if err != nil {
		return nil, fmt.Errorf("get installation: %w", err)
	}

	if inst.Secrets == nil {
		return nil, fmt.Errorf("installation has no secrets configured")
	}

	// Base path stored in APIKey field
	basePath := inst.Secrets.APIKey
	if basePath == "" {
		return nil, fmt.Errorf("installation has no base path configured (expected in api_key field)")
	}

	return NewContainerLister(basePath), nil
}
