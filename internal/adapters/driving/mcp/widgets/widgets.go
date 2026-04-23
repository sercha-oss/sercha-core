package widgets

import (
	"context"
	_ "embed"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// URIs for registered widget resources. Exported so server.go can reference
// them symbolically when attaching _meta.ui.resourceUri to tool definitions.
const (
	SearchURI   = "ui://sercha/search-results"
	DocumentURI = "ui://sercha/document"
)

// HTMLMIMEType is the MCP Apps (ext-apps) profile MIME for app HTML resources.
const HTMLMIMEType = "text/html;profile=mcp-app"

//go:embed search.html
var searchTemplate string

//go:embed document.html
var documentTemplate string

// Rendered HTML is computed once at package init with provider icon data URIs
// substituted in. The alternative (rendering per resources/read) would waste
// CPU for no benefit: icons are static and the set of providers only changes
// on deploy.
var (
	renderedSearchHTML   = renderTemplate(searchTemplate)
	renderedDocumentHTML = renderTemplate(documentTemplate)
)

func renderTemplate(tmpl string) string {
	out := strings.Replace(tmpl, "__ICONS__", iconMapJS(iconDataURIsByProvider()), 1)
	out = strings.Replace(out, "__PROVIDER_LABELS__", labelMapJS(providerLabelsByType()), 1)
	return out
}

// RegisterAll registers every widget resource on the given MCP server.
// Call once at server construction, before returning to callers.
func RegisterAll(server *mcpsdk.Server) {
	addHTMLResource(server, SearchURI, "Sercha search results widget", renderedSearchHTML)
	addHTMLResource(server, DocumentURI, "Sercha document widget", renderedDocumentHTML)
}

func addHTMLResource(server *mcpsdk.Server, uri, name, html string) {
	server.AddResource(
		&mcpsdk.Resource{URI: uri, Name: name, MIMEType: HTMLMIMEType},
		func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
			return &mcpsdk.ReadResourceResult{
				Contents: []*mcpsdk.ResourceContents{
					{URI: uri, MIMEType: HTMLMIMEType, Text: html},
				},
			}, nil
		},
	)
}

// MetaFor returns the _meta map to attach to a tool definition so MCP Apps
// hosts render the given widget. Per the 2026-01-26 spec the host reads this
// from Tool.Meta (on tools/list), not CallToolResult.Meta.
func MetaFor(resourceURI string) mcpsdk.Meta {
	return mcpsdk.Meta{
		"ui": map[string]any{
			"resourceUri": resourceURI,
		},
	}
}

// MetaForSearch is a convenience for the search tool's _meta.
func MetaForSearch() mcpsdk.Meta { return MetaFor(SearchURI) }

// MetaForDocument is a convenience for the get_document tool's _meta.
func MetaForDocument() mcpsdk.Meta { return MetaFor(DocumentURI) }
