package domain

// ProviderMetadata describes presentation details for a ProviderType.
// This is the single source of truth adapters consult when they need to
// render provider-facing UI (icons, display names) — MCP widgets, the
// Next.js admin UI, and any future adapter should all read from here
// rather than hard-coding provider strings.
//
// IconID is a stable identifier, not a filesystem path. Each adapter maps
// IconID to its own asset source: the MCP adapter serves a PNG from embed.FS,
// the Next.js UI bundles an SVG, a hypothetical CLI renders an ASCII glyph.
// Keeping the domain ignorant of paths preserves the hexagonal boundary.
type ProviderMetadata struct {
	Type        ProviderType
	DisplayName string
	Platform    PlatformType
	IconID      string
}

// providerMetadata is the authoritative list of provider presentation data.
// To add a new provider: add an entry here, and ship an asset for the new
// IconID in each adapter that renders provider icons.
var providerMetadata = []ProviderMetadata{
	{
		Type:        ProviderTypeGitHub,
		DisplayName: "GitHub",
		Platform:    PlatformGitHub,
		IconID:      "github",
	},
	{
		Type:        ProviderTypeNotion,
		DisplayName: "Notion",
		Platform:    PlatformNotion,
		IconID:      "notion",
	},
	{
		Type:        ProviderTypeOneDrive,
		DisplayName: "OneDrive",
		Platform:    PlatformMicrosoft,
		IconID:      "onedrive",
	},
	{
		Type:        ProviderTypeLocalFS,
		DisplayName: "Local files",
		Platform:    PlatformLocalFS,
		IconID:      "localfs",
	},
}

// AllProviderMetadata returns metadata for every known provider in a stable
// order. Callers must not mutate the returned slice.
func AllProviderMetadata() []ProviderMetadata {
	out := make([]ProviderMetadata, len(providerMetadata))
	copy(out, providerMetadata)
	return out
}

// MetadataFor returns metadata for a ProviderType. The second return value
// is false if the provider is not registered.
func MetadataFor(p ProviderType) (ProviderMetadata, bool) {
	for _, m := range providerMetadata {
		if m.Type == p {
			return m, true
		}
	}
	return ProviderMetadata{}, false
}
