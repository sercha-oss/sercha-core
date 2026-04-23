// Package widgets wires MCP Apps (ext-apps) interactive UI into the Sercha
// MCP server. Two widgets are registered: one for the `search` tool (compact
// list of result rows) and one for `get_document` (single inline document
// card). Both are served as ui:// resources whose MIME type is the MCP Apps
// app profile, and the associated tools advertise the resource URIs via
// _meta.ui.resourceUri so compatible hosts (Claude, VS Code, Goose, etc.)
// render them in sandboxed iframes.
//
// Clients without MCP Apps support ignore _meta and keep rendering the
// tool's TextContent unchanged. Nothing in this package is on the hot path
// of tool execution — resources are registered once at server startup and
// the HTML is rendered once at package init.
//
// Adding a new provider icon:
//  1. Add an entry to domain.providerMetadata with the new IconID.
//  2. Drop a <IconID>.png into internal/assets/provider_icons/.
//
// Adding a new widget (e.g. for a future `list_sources` richer view):
//  1. Add the HTML next to search.html / document.html.
//  2. Extend widgets.go to register the resource and expose a MetaFor*
//     helper returning the right resource URI.
package widgets
