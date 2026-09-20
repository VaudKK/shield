//go:build !cgo

package ocr

import (
	"context"
	"fmt"
	"strings"
)

// TesseractService is a stub used only when building without cgo (see
// tesseract_cgo.go). It lets the rest of the codebase build and test
// locally on machines without a working cgo toolchain; the real OCR
// backend is always used in the deployed (Docker) build.
type TesseractService struct{}

func NewTesseractService() *TesseractService {
	return &TesseractService{}
}

func (s *TesseractService) ExtractText(_ context.Context, _ []byte, mimeType string) (Result, error) {
	if !strings.HasPrefix(mimeType, "image/") {
		return Result{Applicable: false}, nil
	}
	return Result{}, fmt.Errorf("OCR is unavailable: this build was compiled without cgo/Tesseract support")
}

func (s *TesseractService) ExtractWordBoxes(_ context.Context, _ []byte, mimeType string) ([]BoxedWord, error) {
	if !strings.HasPrefix(mimeType, "image/") {
		return nil, nil
	}
	return nil, fmt.Errorf("OCR is unavailable: this build was compiled without cgo/Tesseract support")
}
