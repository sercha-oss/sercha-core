package mcp

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestServerIcons_EmitsBothFormatsPNGFirst(t *testing.T) {
	icons := serverIcons()
	if len(icons) != 2 {
		t.Fatalf("serverIcons() returned %d icons, want 2 (PNG + SVG)", len(icons))
	}

	// PNG must be first so hosts that don't accept SVG see a valid icon.
	png := icons[0]
	if png.MIMEType != "image/png" {
		t.Errorf("icons[0].MIMEType = %q, want image/png", png.MIMEType)
	}
	if !strings.HasPrefix(png.Source, "data:image/png;base64,") {
		t.Fatalf("icons[0].Source does not start with PNG data URI prefix")
	}
	pngBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(png.Source, "data:image/png;base64,"))
	if err != nil {
		t.Fatalf("PNG base64 decode failed: %v", err)
	}
	if len(pngBytes) < 8 || string(pngBytes[1:4]) != "PNG" {
		t.Errorf("decoded payload is not a PNG (magic bytes missing)")
	}

	svg := icons[1]
	if svg.MIMEType != "image/svg+xml" {
		t.Errorf("icons[1].MIMEType = %q, want image/svg+xml", svg.MIMEType)
	}
	svgBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(svg.Source, "data:image/svg+xml;base64,"))
	if err != nil {
		t.Fatalf("SVG base64 decode failed: %v", err)
	}
	if !strings.Contains(string(svgBytes), "<svg") {
		t.Errorf("decoded SVG payload has no <svg tag")
	}

	for i, icon := range icons {
		if len(icon.Sizes) != 1 || icon.Sizes[0] != "any" {
			t.Errorf("icons[%d].Sizes = %v, want [any]", i, icon.Sizes)
		}
	}
}
