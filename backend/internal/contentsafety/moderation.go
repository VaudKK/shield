package contentsafety

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/storage"
)

// BoundingBox is a detection's location within an image, in pixels.
type BoundingBox struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// ModerationResult is the outcome of one content-safety scan. Status is
// always "safe", "review", or "sensitive" (see DecideStatus) — a
// moderation result changes what happens next, never whether the evidence
// is kept.
type ModerationResult struct {
	Status      string
	Confidence  float64
	Labels      []string
	BoundingBox []BoundingBox
}

// ModerationService scans a file already in object storage for sensitive
// content. file is the storage key the file was written to under, so the
// scan always runs against the actual stored original, never a second,
// independently supplied copy of it.
type ModerationService interface {
	Scan(ctx context.Context, file string) (*ModerationResult, error)
}

// NudeNetModerationService implements ModerationService on top of a
// Classifier (the NudeNet HTTP client) and object storage: it fetches the
// stored file by key, classifies it, and turns the classification into a
// ModerationResult via the same policy (DecideStatus) the rest of Shield
// uses to decide an evidence status.
type NudeNetModerationService struct {
	classifier Classifier
	storage    storage.Storage
}

func NewNudeNetModerationService(classifier Classifier, store storage.Storage) *NudeNetModerationService {
	return &NudeNetModerationService{classifier: classifier, storage: store}
}

func (m *NudeNetModerationService) Scan(ctx context.Context, file string) (*ModerationResult, error) {
	data, err := m.storage.GetObject(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("fetch file for moderation scan: %w", err)
	}

	classification, err := m.classifier.Classify(ctx, data, file)
	if err != nil {
		return nil, fmt.Errorf("classify file: %w", err)
	}

	status := DecideStatus(classification)

	labels := make([]string, len(classification.Labels))
	var boxes []BoundingBox
	for i, l := range classification.Labels {
		labels[i] = l.Name
		if l.Box != nil {
			boxes = append(boxes, *l.Box)
		}
	}

	return &ModerationResult{
		Status:      string(status),
		Confidence:  classification.MaxSensitiveScore,
		Labels:      labels,
		BoundingBox: boxes,
	}, nil
}
