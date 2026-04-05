package driving

import (
	"context"
)

// SetupStatusResponse represents the setup state for FTUE flow
type SetupStatusResponse struct {
	SetupComplete  bool `json:"setup_complete"`
	HasUsers       bool `json:"has_users"`
	HasSources     bool `json:"has_sources"`
	VespaConnected bool `json:"vespa_connected"`
}

// SetupService provides setup and initialization information
type SetupService interface {
	// GetStatus returns the current setup status for FTUE flow
	GetStatus(ctx context.Context) (*SetupStatusResponse, error)
}
