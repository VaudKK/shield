//go:build cgo

// Tesseract OCR support requires CGO and the tesseract-ocr / libtesseract
// development libraries at build time (see backend/Dockerfile). Building
// without cgo available (e.g. some local Windows setups without a 64-bit
// gcc) falls back to tesseract_nocgo.go instead of failing the whole build.
package ocr

import (
	"context"
	"fmt"
	"strings"

	gosseract "github.com/otiai10/gosseract/v2"
)

// TesseractService extracts text from image evidence using a local
// Tesseract OCR engine (no external network call).
type TesseractService struct{}

func NewTesseractService() *TesseractService {
	return &TesseractService{}
}

func (s *TesseractService) ExtractText(_ context.Context, data []byte, mimeType string) (Result, error) {
	if !strings.HasPrefix(mimeType, "image/") {
		return Result{Applicable: false}, nil
	}

	client := gosseract.NewClient()
	defer client.Close()

	if err := client.SetImageFromBytes(data); err != nil {
		return Result{}, fmt.Errorf("load image for OCR: %w", err)
	}

	text, err := client.Text()
	if err != nil {
		return Result{}, fmt.Errorf("run OCR: %w", err)
	}

	return Result{Text: strings.TrimSpace(text), Applicable: true}, nil
}

// ExtractWordBoxes locates each recognized word's pixel position, enabling
// true pixel redaction (drawing over PII where it actually appears) rather
// than redacting only a text transcript.
func (s *TesseractService) ExtractWordBoxes(_ context.Context, data []byte, mimeType string) ([]BoxedWord, error) {
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, nil
	}

	client := gosseract.NewClient()
	defer client.Close()

	if err := client.SetImageFromBytes(data); err != nil {
		return nil, fmt.Errorf("load image for OCR: %w", err)
	}

	boxes, err := client.GetBoundingBoxes(gosseract.RIL_WORD)
	if err != nil {
		return nil, fmt.Errorf("get word boxes: %w", err)
	}

	out := make([]BoxedWord, len(boxes))
	for i, b := range boxes {
		out[i] = BoxedWord{Text: b.Word, Rect: b.Box}
	}
	return out, nil
}
