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

// redactPDFFile redacts a PDF in place via the configured pdfredact.Service
// (real content-stream removal, not an overlay) when one is available,
// falling back to a redacted plain-text transcript when it isn't configured
// or the call fails — the same graceful-degradation pattern the rest of
// this app uses for its other optional external services (content safety,
// AI analysis). The returned method string records which path was actually
// taken, for the audit trail.
func (s *Service) redactPDFFile(
	ctx context.Context,
	evidenceID uuid.UUID,
	data []byte,
	accepted []domain.PIIDetection,
	applied map[uuid.UUID]bool,
) (redacted []byte, mime, ext, method string, err error) {
	if s.pdfRedactor != nil {
		values := make([]string, len(accepted))
		for i, item := range accepted {
			values[i] = item.Value
		}
		result, redactErr := s.pdfRedactor.Redact(ctx, data, values)
		if redactErr == nil {
			for _, item := range accepted {
				applied[item.ID] = result.Applied[item.Value]
			}
			return result.PDF, "application/pdf", ".pdf", "pdf_inplace", nil
		}
		// Fall through to the text-transcript fallback rather than failing
		// the whole redaction — the evidence is still safely stored, and a
		// weaker-but-real redaction beats none.
	}

	redacted, mime, ext, err = s.redactTextTranscript(ctx, evidenceID, accepted, applied)
	return redacted, mime, ext, "pdf_text_transcript_fallback", err
}

// redactTextTranscript is the fallback for non-image evidence (PDFs) when
// no PDF redaction service is configured or reachable: it produces a
// redacted plain-text copy of the OCR'd content, not a visually redacted
// document.
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
