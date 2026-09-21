package redaction

import (
	"image"
	"strings"

	"github.com/VaudKK/shield/backend/internal/ocr"
)

// RedactImageForValues draws black boxes over every located occurrence of
// each value in data (decoded as an image), returning the redacted PNG
// bytes and which values were actually found and covered. Exported so
// internal/disclosure can reuse the same real pixel-redaction logic
// (word-box matching + drawing) that the per-evidence Redact flow uses,
// rather than duplicating it.
func RedactImageForValues(data []byte, words []ocr.BoxedWord, values []string) ([]byte, map[string]bool, error) {
	applied := make(map[string]bool, len(values))
	var regions []image.Rectangle

	for _, v := range values {
		found := findRegions(words, v)
		applied[v] = len(found) > 0
		regions = append(regions, found...)
	}

	redacted, err := redactImage(data, regions)
	if err != nil {
		return nil, nil, err
	}
	return redacted, applied, nil
}

// RedactRegions draws black boxes over the given regions and re-encodes the
// result as PNG. Exported so internal/disclosure can layer face-blur boxes
// (from internal/faceblur) onto an image using the same drawing primitive
// PII redaction uses, and so a plain re-encode (regions == nil) can be used
// to strip file metadata without any boxes drawn.
func RedactRegions(data []byte, regions []image.Rectangle) ([]byte, error) {
	return redactImage(data, regions)
}

// RedactTextForValues replaces every occurrence of each value with
// "[REDACTED]" and reports which values were actually found.
func RedactTextForValues(text string, values []string) (string, map[string]bool) {
	applied := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" {
			applied[v] = false
			continue
		}
		if strings.Contains(text, v) {
			text = strings.ReplaceAll(text, v, "[REDACTED]")
			applied[v] = true
		} else {
			applied[v] = false
		}
	}
	return text, applied
}
