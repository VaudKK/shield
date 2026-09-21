// Package pdfredact calls out to a self-hosted PDF redaction service that
// can do what the Go side can't on its own: edit a PDF's actual content
// stream. internal/redaction can draw real pixel boxes over an image
// because it has OCR word coordinates and a raster library; a PDF is a
// structured document format, and removing text "in place" means editing
// that structure, not painting over it — this package is that missing
// capability, not a policy decision about what gets redacted (the caller
// already knows that from the accepted-PII review workflow).
package pdfredact

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Result is the outcome of one redaction call: the redacted PDF bytes, and
// which of the requested values were actually found and covered. A value
// not found is not an error — the same "not located, not applied" honesty
// the image-redaction path already uses.
type Result struct {
	PDF     []byte
	Applied map[string]bool
}

// Service redacts every occurrence of each value from a PDF, in place —
// the underlying text is removed, not just visually covered.
type Service interface {
	Redact(ctx context.Context, pdfBytes []byte, values []string) (*Result, error)
}

// redactResponse mirrors pdf-redact-service's RedactResponse.
type redactResponse struct {
	PDFBase64 string          `json:"pdf_base64"`
	Applied   map[string]bool `json:"applied"`
}

func decodeRedactResponse(body []byte) (*Result, error) {
	var parsed redactResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode redact response: %w", err)
	}
	pdfBytes, err := base64.StdEncoding.DecodeString(parsed.PDFBase64)
	if err != nil {
		return nil, fmt.Errorf("decode redacted pdf: %w", err)
	}
	return &Result{PDF: pdfBytes, Applied: parsed.Applied}, nil
}
