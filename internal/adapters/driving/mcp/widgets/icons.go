package widgets

import (
	"encoding/base64"
	"log"
	"sort"
	"strings"

	"github.com/sercha-oss/sercha-core/internal/assets"
	"github.com/sercha-oss/sercha-core/internal/core/domain"
)

// iconDataURIsByProvider returns a {providerType: dataURI} map covering every
// provider in domain.AllProviderMetadata that has a matching icon asset.
// Providers without an asset are skipped silently (the widget falls back to
// a letter chip at render time) — a missing-icon warning is logged once at
// package init so it surfaces during development without breaking startup.
func iconDataURIsByProvider() map[string]string {
	out := map[string]string{}
	for _, m := range domain.AllProviderMetadata() {
		data, err := assets.ReadProviderIcon(m.IconID)
		if err != nil {
			log.Printf("mcp/widgets: no icon asset for provider %q (icon_id=%q): %v",
				m.Type, m.IconID, err)
			continue
		}
		out[string(m.Type)] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	}
	return out
}

// providerLabelsByType returns a {providerType: displayName} map for every
// known provider. Used to substitute a PROVIDER_LABELS table into widgets
// that render human-facing source labels.
func providerLabelsByType() map[string]string {
	out := map[string]string{}
	for _, m := range domain.AllProviderMetadata() {
		out[string(m.Type)] = m.DisplayName
	}
	return out
}

// labelMapJS renders a {providerType: displayName} map as JS object-literal
// entries. Order is deterministic.
func labelMapJS(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",\n    ")
		}
		b.WriteString(jsKey(k))
		b.WriteString(": ")
		b.WriteString(jsString(labels[k]))
	}
	return b.String()
}

// iconMapJS renders the icon map as JavaScript object-literal entries for
// substitution into a widget template's __ICONS__ placeholder. Entries are
// emitted in provider_type order so the rendered HTML is deterministic.
func iconMapJS(icons map[string]string) string {
	if len(icons) == 0 {
		return ""
	}
	keys := make([]string, 0, len(icons))
	for k := range icons {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",\n    ")
		}
		b.WriteString(jsKey(k))
		b.WriteString(": ")
		b.WriteString(jsString(icons[k]))
	}
	return b.String()
}

// jsKey emits a safe JS object key. ProviderType values are known-safe
// identifiers today (github/notion/onedrive/localfs) but always-quote to
// keep future providers with hyphens or dots working without thought.
func jsKey(s string) string { return jsString(s) }

// jsString emits a double-quoted JS string literal. Only characters we
// actually emit (ASCII from ProviderType + base64 data URIs) are handled.
func jsString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
