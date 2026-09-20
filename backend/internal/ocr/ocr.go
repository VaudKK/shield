// Package ocr extracts text from evidence files. Implementations are kept
// behind the Service interface so the extraction engine can be swapped
// without touching callers, and so it stays independent of the AI service —
// OCR is deterministic text extraction, not a model call.
package ocr

import (
	"context"
	"image"
)

type Result struct {
	Text string
	// Applicable is false when the file type has no text-extraction path in
	// this implementation (e.g. an image format OCR doesn't support). The
	// caller should not treat that as an error.
	Applicable bool
}

type Service interface {
	ExtractText(ctx context.Context, data []byte, mimeType string) (Result, error)
}

// BoxedWord is one recognized word and its pixel location on the page.
type BoxedWord struct {
	Text string
	Rect image.Rectangle
}

// WordBoxService is an optional capability: locating where recognized text
// sits on the page, not just what it says. Only image-based OCR backends
// implement this — it's what makes true pixel redaction (drawing over PII
// in its actual location) possible, as opposed to redacting a text
// transcript.
type WordBoxService interface {
	ExtractWordBoxes(ctx context.Context, data []byte, mimeType string) ([]BoxedWord, error)
}
