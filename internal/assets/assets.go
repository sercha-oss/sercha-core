// Package assets holds binary assets that multiple adapters embed and serve.
// Keeping them here — rather than duplicated under each adapter's directory —
// means a new connector ships one icon file that every adapter picks up.
//
// This package must not import domain or ports; it's a leaf package shared
// across adapters. Any lookup by ProviderType lives in the calling adapter.
package assets

import (
	"embed"
	"fmt"
)

//go:embed provider_icons/*.png
var providerIconFS embed.FS

// ReadProviderIcon returns the raw bytes of the provider icon for iconID.
// Returns an error if no asset is present for that IconID.
func ReadProviderIcon(iconID string) ([]byte, error) {
	if iconID == "" {
		return nil, fmt.Errorf("provider icon: empty iconID")
	}
	data, err := providerIconFS.ReadFile("provider_icons/" + iconID + ".png")
	if err != nil {
		return nil, fmt.Errorf("provider icon %q: %w", iconID, err)
	}
	return data, nil
}

//go:embed sercha_icon.svg
var serchaIconSVG []byte

//go:embed sercha_icon.png
var serchaIconPNG []byte

// SerchaIconSVG returns the Sercha brand icon as raw SVG bytes. Vector form;
// suitable for any renderer that supports SVG.
func SerchaIconSVG() []byte {
	out := make([]byte, len(serchaIconSVG))
	copy(out, serchaIconSVG)
	return out
}

// SerchaIconPNG returns the Sercha brand icon as raw PNG bytes. Raster form
// for hosts that don't accept SVG (most MCP clients today restrict to raster
// to avoid script-embedded SVG payloads).
func SerchaIconPNG() []byte {
	out := make([]byte, len(serchaIconPNG))
	copy(out, serchaIconPNG)
	return out
}
