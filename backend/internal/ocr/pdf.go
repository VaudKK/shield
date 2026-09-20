package ocr

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/ledongthuc/pdf"
)

// PDFTextService extracts embedded text from PDFs directly, without OCR.
// Scanned or image-only PDFs (no embedded text layer) yield an empty
// result rather than an error — rendering pages to images and running OCR
// on them is out of scope for this MVP; see README for the limitation.
type PDFTextService struct{}

func NewPDFTextService() *PDFTextService {
	return &PDFTextService{}
}

func (s *PDFTextService) ExtractText(_ context.Context, data []byte, mimeType string) (Result, error) {
	if mimeType != "application/pdf" {
		return Result{Applicable: false}, nil
	}

	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Result{}, fmt.Errorf("open pdf: %w", err)
	}

	textReader, err := reader.GetPlainText()
	if err != nil {
		return Result{}, fmt.Errorf("extract pdf text: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, textReader); err != nil {
		return Result{}, fmt.Errorf("read extracted pdf text: %w", err)
	}

	return Result{Text: buf.String(), Applicable: true}, nil
}
