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
