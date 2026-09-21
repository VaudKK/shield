package domain

import (
	"time"

	"github.com/google/uuid"
)

// BoundingBox is a detection's location within an image, in pixels.
type BoundingBox struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// ModerationResult is one persisted content-safety scan against a piece of
// evidence. It is kept separate from Evidence.Status (which reflects only
// the current verdict) so the underlying signal — labels, confidence, and
// any bounding boxes — stays auditable. NudeNet is a risk signal, not a
// final decision: nothing reads this table to justify deleting evidence.
type ModerationResult struct {
	ID          uuid.UUID
	EvidenceID  uuid.UUID
	Status      EvidenceStatus // one of EvidenceStatusSafe, EvidenceStatusReview, EvidenceStatusSensitive
	Confidence  float64
	Labels      []string
	BoundingBox []BoundingBox
	CreatedAt   time.Time
}
