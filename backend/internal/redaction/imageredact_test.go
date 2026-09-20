package redaction

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/VaudKK/shield/backend/internal/ocr"
)

func TestFindRegions_SingleWord(t *testing.T) {
	words := []ocr.BoxedWord{
		{Text: "Call", Rect: image.Rect(0, 0, 30, 10)},
		{Text: "jane.doe@example.com", Rect: image.Rect(35, 0, 150, 10)},
		{Text: "now.", Rect: image.Rect(155, 0, 180, 10)},
	}

	regions := findRegions(words, "jane.doe@example.com")
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	if regions[0] != image.Rect(35, 0, 150, 10) {
		t.Errorf("unexpected region: %v", regions[0])
	}
}

func TestFindRegions_MultiWord(t *testing.T) {
	words := []ocr.BoxedWord{
		{Text: "Meeting", Rect: image.Rect(0, 0, 40, 10)},
		{Text: "with", Rect: image.Rect(45, 0, 65, 10)},
		{Text: "Jane", Rect: image.Rect(70, 0, 100, 10)},
		{Text: "Smith", Rect: image.Rect(105, 0, 145, 10)},
		{Text: "today", Rect: image.Rect(150, 0, 185, 10)},
	}

	regions := findRegions(words, "Jane Smith")
	if len(regions) != 1 {
		t.Fatalf("expected 1 region, got %d", len(regions))
	}
	want := image.Rect(70, 0, 145, 10)
	if regions[0] != want {
		t.Errorf("expected union rect %v, got %v", want, regions[0])
	}
}

func TestFindRegions_CaseInsensitiveAndPunctuation(t *testing.T) {
	words := []ocr.BoxedWord{
		{Text: "Contact:", Rect: image.Rect(0, 0, 50, 10)},
		{Text: "JANE.DOE@EXAMPLE.COM,", Rect: image.Rect(55, 0, 180, 10)},
	}

	regions := findRegions(words, "jane.doe@example.com")
	if len(regions) != 1 {
		t.Fatalf("expected 1 region despite case/punctuation differences, got %d", len(regions))
	}
}

func TestFindRegions_NoMatch(t *testing.T) {
	words := []ocr.BoxedWord{
		{Text: "Hello", Rect: image.Rect(0, 0, 30, 10)},
		{Text: "world", Rect: image.Rect(35, 0, 65, 10)},
	}

	regions := findRegions(words, "not-present@example.com")
	if len(regions) != 0 {
		t.Errorf("expected no regions, got %d", len(regions))
	}
}

func TestRedactImage_CoversRegion(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, color.White)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	redacted, err := redactImage(buf.Bytes(), []image.Rectangle{image.Rect(5, 5, 15, 15)})
	if err != nil {
		t.Fatalf("redactImage: %v", err)
	}

	out, err := png.Decode(bytes.NewReader(redacted))
	if err != nil {
		t.Fatalf("decode redacted output: %v", err)
	}

	// Inside the redacted region should now be black.
	r, g, b, _ := out.At(10, 10).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("expected black pixel inside redacted region, got (%d,%d,%d)", r, g, b)
	}

	// Outside the redacted region should remain untouched (white).
	r, g, b, _ = out.At(1, 1).RGBA()
	if r == 0 && g == 0 && b == 0 {
		t.Error("expected pixel outside redacted region to remain unchanged, got black")
	}
}
