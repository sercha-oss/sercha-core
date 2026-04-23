package mcp

import (
	"encoding/base64"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sercha-oss/sercha-core/internal/assets"
)

// serverIcons returns the icon list advertised on the MCP Implementation.
// Hosts that support MCP server icons (Claude Desktop, VS Code) render this
// in their connector UI. Delivered as data URIs so self-hosted deployments
// don't need an extra HTTP route and so the icon works when the host isn't
// on the same network.
//
// We ship PNG first, SVG second. Many MCP hosts (Claude Desktop included as
// of April 2026) refuse SVG for server icons because SVG can embed scripts;
// listing PNG first makes sure the host has something it's willing to render.
// Hosts that prefer SVG are free to pick the second entry.
func serverIcons() []mcpsdk.Icon {
	var icons []mcpsdk.Icon
	if png := assets.SerchaIconPNG(); len(png) > 0 {
		icons = append(icons, mcpsdk.Icon{
			Source:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
			MIMEType: "image/png",
			Sizes:    []string{"any"},
		})
	}
	if svg := assets.SerchaIconSVG(); len(svg) > 0 {
		icons = append(icons, mcpsdk.Icon{
			Source:   "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(svg),
			MIMEType: "image/svg+xml",
			Sizes:    []string{"any"},
		})
	}
	return icons
}
