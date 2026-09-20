package redaction

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
)

// redactImageFile locates each accepted PII value in the image via OCR word
// boxes and draws a black rectangle over every region found, producing a
// PNG regardless of the original format.
func (s *Service) redactImageFile(
	ctx context.Context,
	data []byte,
	mimeType string,
	accepted []domain.PIIDetection,
	applied map[uuid.UUID]bool,
) (redacted []byte, mime, ext string, err error) {
	words, err := s.ocr.ExtractWordBoxes(ctx, data, mimeType)
	if err != nil {
		return nil, "", "", fmt.Errorf("locate text for redaction: %w", err)
	}

	values := make([]string, len(accepted))
	for i, item := range accepted {
		values[i] = item.Value
	}

	redacted, appliedByValue, err := RedactImageForValues(data, words, values)
	if err != nil {
		return nil, "", "", fmt.Errorf("draw redactions: %w", err)
	}
	for _, item := range accepted {
		applied[item.ID] = appliedByValue[item.Value]
	}

	return redacted, "image/png", ".png", nil
}

// redactTextTranscript is the fallback for non-image evidence (PDFs): it
// produces a redacted plain-text copy of the OCR'd content, not a visually
// redacted document. True PDF content redaction — covering the actual
// rendered text in place — needs a PDF-editing library beyond this MVP's
// scope; this is documented in the README as a known limitation.
func (s *Service) redactTextTranscript(
	ctx context.Context,
	evidenceID uuid.UUID,
	accepted []domain.PIIDetection,
	applied map[uuid.UUID]bool,
) (redacted []byte, mime, ext string, err error) {
	analysis, err := s.analysisRepo.GetByEvidenceID(ctx, evidenceID)
	text := ""
	if err == nil {
		text = analysis.OCRText
	}

	values := make([]string, len(accepted))
	for i, item := range accepted {
		values[i] = item.Value
	}

	redactedText, appliedByValue := RedactTextForValues(text, values)
	for _, item := range accepted {
		applied[item.ID] = appliedByValue[item.Value]
	}

	return []byte(redactedText), "text/plain; charset=utf-8", ".txt", nil
}

func bytesReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

func redactedFilename(original, ext string) string {
	base := original
	if i := strings.LastIndex(original, "."); i != -1 {
		base = original[:i]
	}
	return base + "-redacted" + ext
}
