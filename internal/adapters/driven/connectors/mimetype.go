package connectors

import (
	"path/filepath"
	"strings"
)

// mimeTypes maps file extensions to MIME types.
var mimeTypes = map[string]string{
	// Markup / documentation
	".md":       "text/markdown",
	".markdown": "text/markdown",
	".mdx":      "text/markdown",
	".txt":      "text/plain",
	".text":     "text/plain",
	".rst":      "text/x-rst",
	".adoc":     "text/asciidoc",
	".asciidoc": "text/asciidoc",
	// Web
	".html":   "text/html",
	".htm":    "text/html",
	".css":    "text/css",
	".scss":   "text/x-scss",
	".sass":   "text/x-sass",
	".less":   "text/css",
	".js":     "application/javascript",
	".mjs":    "application/javascript",
	".cjs":    "application/javascript",
	".jsx":    "text/javascript-jsx",
	".ts":     "application/typescript",
	".mts":    "application/typescript",
	".cts":    "application/typescript",
	".tsx":    "text/typescript-jsx",
	".vue":    "text/html",
	".svelte": "text/html",
	// Data
	".json":  "application/json",
	".jsonc": "application/json",
	".yaml":  "text/yaml",
	".yml":   "text/yaml",
	".toml":  "text/x-toml",
	".xml":   "application/xml",
	".xsl":   "application/xml",
	".xslt":  "application/xml",
	".csv":   "text/csv",
	".ini":   "text/plain",
	".cfg":   "text/plain",
	".conf":  "text/plain",
	".env":   "text/plain",
	// Programming languages
	".go":    "text/x-go",
	".py":    "text/x-python",
	".pyi":   "text/x-python",
	".rs":    "text/x-rust",
	".java":  "text/x-java",
	".kt":    "text/x-kotlin",
	".kts":   "text/x-kotlin",
	".scala": "text/x-scala",
	".rb":    "text/x-ruby",
	".c":     "text/x-c",
	".h":     "text/x-c",
	".cpp":   "text/x-c++",
	".cc":    "text/x-c++",
	".cxx":   "text/x-c++",
	".hpp":   "text/x-c++",
	".cs":    "text/x-csharp",
	".swift": "text/x-swift",
	".php":   "text/x-php",
	".lua":   "text/x-lua",
	".r":     "text/x-r",
	".pl":    "text/x-perl",
	".pm":    "text/x-perl",
	".ex":    "text/x-elixir",
	".exs":   "text/x-elixir",
	".erl":   "text/x-erlang",
	".hrl":   "text/x-erlang",
	".hs":    "text/x-haskell",
	".clj":   "text/x-clojure",
	".cljs":  "text/x-clojure",
	".dart":  "text/x-dart",
	".zig":   "text/x-zig",
	// Shell / scripts
	".sh":   "text/x-shellscript",
	".bash": "text/x-shellscript",
	".zsh":  "text/x-shellscript",
	".fish": "text/x-shellscript",
	".ps1":  "text/x-powershell",
	// Build / config
	".dockerfile": "text/x-dockerfile",
	".makefile":   "text/x-makefile",
	".tf":         "text/x-hcl",
	".hcl":        "text/x-hcl",
	".proto":      "text/x-protobuf",
	".graphql":    "text/x-graphql",
	".gql":        "text/x-graphql",
	".sql":        "application/sql",
	// Images
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".heic": "image/heic",
	".heif": "image/heif",
	".avif": "image/avif",
	// Binary / archives
	".zip":   "application/zip",
	".tar":   "application/x-tar",
	".gz":    "application/gzip",
	".rar":   "application/vnd.rar",
	".7z":    "application/x-7z-compressed",
	".exe":   "application/x-msdownload",
	".dll":   "application/x-msdownload",
	".so":    "application/x-sharedlib",
	".dylib": "application/x-sharedlib",
	".wasm":  "application/wasm",
	// Fonts
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".eot":   "application/vnd.ms-fontobject",
}

// extensionlessNames maps well-known filenames (without extensions) to MIME types.
var extensionlessNames = map[string]string{
	"dockerfile":    "text/x-dockerfile",
	"containerfile": "text/x-dockerfile",
	"makefile":      "text/x-makefile",
	"gnumakefile":   "text/x-makefile",
	"readme":        "text/plain",
	"license":       "text/plain",
	"licence":       "text/plain",
	"copying":       "text/plain",
	".gitignore":    "text/plain",
	".dockerignore": "text/plain",
	".editorconfig": "text/plain",
}

// GuessMimeType determines the MIME type of a file based on its extension
// or filename. Returns "text/plain" for unrecognised extensions.
func GuessMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}

	base := strings.ToLower(filepath.Base(path))
	if mime, ok := extensionlessNames[base]; ok {
		return mime
	}

	return "text/plain"
}
