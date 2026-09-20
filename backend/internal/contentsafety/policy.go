package contentsafety

import "github.com/VaudKK/shield/backend/internal/domain"

// reviewThreshold is the score below the service's own "sensitive" cutoff
// at which content is still flagged for a human to look at, rather than
// waved through as safe outright.
const reviewThreshold = 0.3

// DecideStatus turns a classification into an evidence status. It never
// returns EvidenceStatusRejected — content safety classification alone is
// not grounds to reject evidence; a human always stays in the loop for
// anything borderline or sensitive.
func DecideStatus(c *Classification) domain.EvidenceStatus {
	if c == nil {
		// Classification failed or was unavailable: default to requiring a
		// human look rather than silently calling it safe.
		return domain.EvidenceStatusReview
	}
	if c.Sensitive {
		return domain.EvidenceStatusSensitive
	}
	if c.MaxSensitiveScore >= reviewThreshold {
		return domain.EvidenceStatusReview
	}
	return domain.EvidenceStatusSafe
}
