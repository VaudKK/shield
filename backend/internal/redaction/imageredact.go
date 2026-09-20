package redaction

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg" // decoder registration for image.Decode
	"image/png"    // also self-registers as a decoder for image.Decode
	"strings"

	"github.com/VaudKK/shield/backend/internal/ocr"

	_ "golang.org/x/image/webp" // decoder registration for image.Decode
)

// redactImage draws solid black rectangles over every region in regions and
// re-encodes the result as PNG, regardless of the original format. PNG is
// lossless and universally supported, which matters more here than
// preserving the original container format for a derived, already-modified
// copy.
func redactImage(data []byte, regions []image.Rectangle) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)

	black := image.NewUniform(color.Black)
	for _, r := range regions {
		draw.Draw(dst, r, black, image.Point{}, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// findRegions locates value within the OCR'd words, matching either a
// single word or a run of up to 4 consecutive words (to catch multi-word
// values like names), case-insensitively and ignoring surrounding
// punctuation. Returns nil if value couldn't be located — the caller
// should treat that as "not applied", not an error; OCR recognition is
// imperfect and a value present in the accepted-PII list isn't guaranteed
// to be found verbatim in the word list.
func findRegions(words []ocr.BoxedWord, value string) []image.Rectangle {
	target := normalizeForMatch(value)
	if target == "" {
		return nil
	}

	var regions []image.Rectangle

	for i := range words {
		if normalizeForMatch(words[i].Text) == target {
			regions = append(regions, words[i].Rect)
			continue
		}

		for span := 2; span <= 4 && i+span <= len(words); span++ {
			var joined strings.Builder
			for j := 0; j < span; j++ {
				if j > 0 {
					joined.WriteByte(' ')
				}
				joined.WriteString(words[i+j].Text)
			}
			if normalizeForMatch(joined.String()) == target {
				regions = append(regions, unionRects(words[i:i+span]))
				break
			}
		}
	}

	return regions
}

func unionRects(words []ocr.BoxedWord) image.Rectangle {
	r := words[0].Rect
	for _, w := range words[1:] {
		r = r.Union(w.Rect)
	}
	return r
}

func normalizeForMatch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimFunc(s, func(r rune) bool {
		return r == '.' || r == ',' || r == ';' || r == ':' || r == '"' || r == '\''
	})
}
