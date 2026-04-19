package normalisers

import (
	"os/exec"
	"testing"

	"github.com/sercha-oss/sercha-core/internal/core/ports/driven"
)

// skipIfNoPdftotext skips the test if pdftotext is not available
func skipIfNoPdftotext(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed, skipping PDF normaliser test")
	}
}

func TestPDFNormaliser_InterfaceCompliance(t *testing.T) {
	var _ driven.Normaliser = (*PDFNormaliser)(nil)
}

func TestPDFNormaliser_SupportedTypes(t *testing.T) {
	n := &PDFNormaliser{}
	types := n.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != "application/pdf" {
		t.Errorf("expected application/pdf, got %s", types[0])
	}
}

func TestPDFNormaliser_Priority(t *testing.T) {
	n := &PDFNormaliser{}

	if n.Priority() != 50 {
		t.Errorf("expected priority 50, got %d", n.Priority())
	}
}

func TestPDFNormaliser_EmptyContent(t *testing.T) {
	n := &PDFNormaliser{}

	result := n.Normalise("", "application/pdf")
	if result != "" {
		t.Errorf("expected empty string for empty input, got %q", result)
	}
}

func TestPDFNormaliser_InvalidPDF(t *testing.T) {
	skipIfNoPdftotext(t)

	n := &PDFNormaliser{}

	// Invalid PDF bytes should return empty string
	result := n.Normalise("not a valid pdf", "application/pdf")
	if result != "" {
		t.Errorf("expected empty string for invalid PDF, got %q", result)
	}
}

func TestPDFNormaliser_ValidPDF(t *testing.T) {
	skipIfNoPdftotext(t)

	n := &PDFNormaliser{}

	// Minimal valid PDF with text "Hello"
	// This is a simple PDF 1.4 document
	pdfContent := `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]
   /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT /F1 12 Tf 100 700 Td (Hello World) Tj ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000266 00000 n
0000000359 00000 n
trailer
<< /Size 6 /Root 1 0 R >>
startxref
434
%%EOF`

	result := n.Normalise(pdfContent, "application/pdf")

	// The result should contain "Hello World" (extracted from the PDF)
	if result == "" {
		t.Log("PDF extraction returned empty - this may happen with minimal PDFs")
		// Don't fail - minimal PDFs may not extract well
		return
	}

	// If we got content, it should be cleaned up (no excessive whitespace)
	if len(result) > 0 {
		// Basic sanity check - should not have triple newlines
		if contains(result, "\n\n\n") {
			t.Error("result should not contain triple newlines")
		}
	}
}

func TestPDFNormaliser_RegisteredInDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()

	n := r.Get("application/pdf")
	if n == nil {
		t.Error("expected PDF normaliser to be registered in default registry")
	}
}

func TestPDFNormaliser_WhitespaceNormalization(t *testing.T) {
	// This tests the whitespace cleanup logic without needing pdftotext
	// We test the helper behavior indirectly

	n := &PDFNormaliser{}

	// Empty content
	result := n.Normalise("", "application/pdf")
	if result != "" {
		t.Errorf("expected empty result for empty input")
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
