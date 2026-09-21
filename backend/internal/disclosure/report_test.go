package disclosure

import (
	"strings"
	"testing"
)

func TestRenderReport(t *testing.T) {
	data := ReportData{
		Title:           "Test Package",
		GeneratedAt:     "2026-01-01 00:00:00 UTC",
		IncludeSummary:  true,
		IncludeTimeline: true,
		IncludePhotos:   true,
		Protections:     []string{"Phone numbers removed", "Email addresses removed"},
		Evidence: []ReportEvidence{
			{
				Title:          "Photo 1",
				Filename:       "photo1.png",
				SHA256:         "abc123",
				UploadedAt:     "2026-01-01 00:00:00 UTC",
				Summary:        "The document appears to show a message exchange.",
				HasSummary:     true,
				RedactionNotes: []string{"email: redacted", "phone_number: could not be located in the image, not redacted"},
				IncludedAs:     "01-photo1.png",
			},
		},
		Timeline: []ReportTimelineEntry{
			{Date: "2026-01-01", Description: "Something happened", EvidenceTitle: "Photo 1"},
		},
	}

	out, err := renderReport(data)
	if err != nil {
		t.Fatalf("renderReport: %v", err)
	}

	html := string(out)
	for _, want := range []string{
		"Test Package",
		"Shield does not determine whether an incident occurred",
		"Phone numbers removed",
		"Photo 1",
		"abc123",
		"01-photo1.png",
		"could not be located in the image, not redacted",
		"Something happened",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("expected rendered report to contain %q", want)
		}
	}
}

func TestRenderReport_FaceBlurProtectionLabel(t *testing.T) {
	data := ReportData{
		Title:       "Test",
		GeneratedAt: "now",
		Protections: []string{"Faces blurred where detected (automated — always verify manually before sharing)"},
		Evidence: []ReportEvidence{
			{Title: "Photo 1", RedactionNotes: []string{"2 face(s) detected and blurred automatically — always visually confirm before sharing."}},
		},
	}

	out, err := renderReport(data)
	if err != nil {
		t.Fatalf("renderReport: %v", err)
	}

	html := string(out)
	if !strings.Contains(html, "Faces blurred where detected") {
		t.Error("expected the face-blur protection label in the report")
	}
	if !strings.Contains(html, "2 face(s) detected and blurred") {
		t.Error("expected the per-evidence face-blur note in the report")
	}
}
