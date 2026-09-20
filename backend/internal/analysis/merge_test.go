package analysis

import (
	"testing"

	"github.com/VaudKK/shield/backend/internal/ai"
	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/pii"
	"github.com/google/uuid"
)

func TestMergeDetections(t *testing.T) {
	evidenceID := uuid.New()

	regexHits := []pii.Detection{
		{Type: "email", Value: "jane@example.com", Location: "..."},
		{Type: "phone_number", Value: "+1 555-123-4567", Location: "..."},
	}
	aiHits := []ai.PIIEntry{
		{Type: "email", Value: "jane@example.com", Location: "duplicate of regex hit"},
		{Type: "name", Value: "Jane Doe", Location: "..."},
	}

	got := mergeDetections(evidenceID, regexHits, aiHits)

	if len(got) != 3 {
		t.Fatalf("expected 3 merged detections (duplicate email collapsed), got %d: %+v", len(got), got)
	}

	byValue := map[string]domain.PIIDetection{}
	for _, d := range got {
		byValue[d.Value] = d
	}

	email, ok := byValue["jane@example.com"]
	if !ok {
		t.Fatal("expected email detection to survive merge")
	}
	if email.DetectionMethod != domain.PIIDetectionMethodRegex {
		t.Errorf("expected duplicate email to keep the regex detection method, got %q", email.DetectionMethod)
	}

	name, ok := byValue["Jane Doe"]
	if !ok {
		t.Fatal("expected AI-only name detection to survive merge")
	}
	if name.DetectionMethod != domain.PIIDetectionMethodAI {
		t.Errorf("expected name detection method to be ai, got %q", name.DetectionMethod)
	}

	for _, d := range got {
		if d.EvidenceID != evidenceID {
			t.Errorf("expected evidence ID %v on all detections, got %v", evidenceID, d.EvidenceID)
		}
		if d.Status != domain.PIIStatusDetected {
			t.Errorf("expected status detected, got %q", d.Status)
		}
	}
}

func TestMergeDetections_SkipsEmptyAIValues(t *testing.T) {
	got := mergeDetections(uuid.New(), nil, []ai.PIIEntry{{Type: "name", Value: "  "}})
	if len(got) != 0 {
		t.Errorf("expected empty/whitespace-only AI values to be skipped, got %+v", got)
	}
}
