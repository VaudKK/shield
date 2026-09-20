package contentsafety

import (
	"testing"

	"github.com/VaudKK/shield/backend/internal/domain"
)

func TestDecideStatus(t *testing.T) {
	tests := []struct {
		name string
		c    *Classification
		want domain.EvidenceStatus
	}{
		{"nil classification defaults to review", nil, domain.EvidenceStatusReview},
		{"no detections is safe", &Classification{MaxSensitiveScore: 0}, domain.EvidenceStatusSafe},
		{"low score is safe", &Classification{MaxSensitiveScore: 0.1}, domain.EvidenceStatusSafe},
		{"borderline score needs review", &Classification{MaxSensitiveScore: 0.4}, domain.EvidenceStatusReview},
		{"sensitive flag wins regardless of score", &Classification{Sensitive: true, MaxSensitiveScore: 0.61}, domain.EvidenceStatusSensitive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecideStatus(tt.c)
			if got != tt.want {
				t.Errorf("DecideStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
