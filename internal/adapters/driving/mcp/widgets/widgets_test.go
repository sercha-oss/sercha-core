package widgets

import (
	"context"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// TestRenderedHTML_HasIconForEveryProviderWithAsset is the regression fence:
// every ProviderType in the domain whose IconID has an asset file must appear
// as an icon key in both widget HTMLs. If a new provider is added to the
// domain without shipping an icon, iconDataURIsByProvider logs and skips it
// (intentional graceful degrade). If a new provider is added WITH an icon,
// this test ensures the rendered widget picks it up without further wiring.
func TestRenderedHTML_HasIconForEveryProviderWithAsset(t *testing.T) {
	icons := iconDataURIsByProvider()
	if len(icons) == 0 {
		t.Fatal("no provider icons loaded; did assets/provider_icons/ move?")
	}

	for _, html := range map[string]string{
		"search":   renderedSearchHTML,
		"document": renderedDocumentHTML,
	} {
		for providerType := range icons {
			// Keys appear quoted in the rendered JS map ("github": "data:...").
			needle := "\"" + providerType + "\":"
			if !strings.Contains(html, needle) {
				t.Errorf("rendered HTML missing icon key for provider %q (needle=%q)",
					providerType, needle)
			}
		}
		if strings.Contains(html, "__ICONS__") {
			t.Error("rendered HTML still contains __ICONS__ placeholder")
		}
		if !strings.Contains(html, "data:image/png;base64,") {
			t.Error("rendered HTML has no base64 PNG data URI")
		}
	}
}

func TestRenderedDocumentHTML_HasLabelForEveryProvider(t *testing.T) {
	for _, m := range domain.AllProviderMetadata() {
		needle := "\"" + string(m.Type) + "\": \"" + m.DisplayName + "\""
		if !strings.Contains(renderedDocumentHTML, needle) {
			t.Errorf("document widget missing label entry %q", needle)
		}
	}
	if strings.Contains(renderedDocumentHTML, "__PROVIDER_LABELS__") {
		t.Error("document widget still contains __PROVIDER_LABELS__ placeholder")
	}
}

func TestMetaFor_MatchesMCPAppsSpec(t *testing.T) {
	cases := map[string]struct {
		meta mcpsdk.Meta
		want string
	}{
		"search":   {MetaForSearch(), SearchURI},
		"document": {MetaForDocument(), DocumentURI},
	}
	for name, c := range cases {
		ui, ok := c.meta["ui"].(map[string]any)
		if !ok {
			t.Errorf("%s: _meta.ui missing or wrong type: %T", name, c.meta["ui"])
			continue
		}
		if got, _ := ui["resourceUri"].(string); got != c.want {
			t.Errorf("%s: _meta.ui.resourceUri = %q, want %q", name, got, c.want)
		}
	}
}

func TestRegisterAll_RegistersBothResources(t *testing.T) {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "test", Version: "0.0.0"}, nil)
	RegisterAll(server)

	// ReadResource via the server's in-memory path by constructing and
	// running a client connected via in-memory transport.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	for _, uri := range []string{SearchURI, DocumentURI} {
		res, err := session.ReadResource(ctx, &mcpsdk.ReadResourceParams{URI: uri})
		if err != nil {
			t.Errorf("ReadResource(%s) failed: %v", uri, err)
			continue
		}
		if len(res.Contents) != 1 {
			t.Errorf("ReadResource(%s) returned %d contents, want 1", uri, len(res.Contents))
			continue
		}
		c := res.Contents[0]
		if c.MIMEType != HTMLMIMEType {
			t.Errorf("ReadResource(%s) MIME = %q, want %q", uri, c.MIMEType, HTMLMIMEType)
		}
		if !strings.HasPrefix(strings.TrimSpace(c.Text), "<!doctype html>") {
			t.Errorf("ReadResource(%s) body does not start with <!doctype html>", uri)
		}
	}
}
