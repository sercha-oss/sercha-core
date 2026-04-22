package normalisers

import (
	"strings"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
	"github.com/xuri/excelize/v2"
)

// Verify interface compliance
var _ driven.Normaliser = (*XlsxNormaliser)(nil)

// XlsxNormaliser extracts text content from XLSX files using excelize.
// It extracts text from all sheets with tab-separated cell values.
type XlsxNormaliser struct{}

func (n *XlsxNormaliser) Normalise(content string, mimeType string) string {
	if content == "" {
		return ""
	}

	// Open the XLSX file from the content string
	f, err := excelize.OpenReader(strings.NewReader(content))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	// Extract text from all sheets
	var result strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}

		for _, row := range rows {
			result.WriteString(strings.Join(row, "\t"))
			result.WriteString("\n")
		}
	}

	// Clean up whitespace
	text := result.String()

	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse multiple spaces
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	// Remove excessive blank lines
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(text)
}

func (n *XlsxNormaliser) SupportedTypes() []string {
	return []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}
}

func (n *XlsxNormaliser) Priority() int {
	return 50 // Medium priority - format-specific
}
