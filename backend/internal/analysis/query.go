package analysis

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
)

// Get returns the evidence's analysis, timeline, and PII detections
// without re-running the pipeline. Returns domain.ErrNotFound if the
// evidence doesn't exist, isn't owned by ownerID, or hasn't been analyzed
// yet.
func (s *Service) Get(ctx context.Context, evidenceID, ownerID uuid.UUID) (*Result, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID); err != nil {
		return nil, err
	}

	a, err := s.analysis.GetByEvidenceID(ctx, evidenceID)
	if err != nil {
		return nil, err
	}
	timeline, err := s.timeline.ListByEvidence(ctx, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list timeline: %w", err)
	}
	piiDetections, err := s.piiRepo.ListByEvidence(ctx, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list pii: %w", err)
	}

	return &Result{Analysis: a, Timeline: timeline, PII: piiDetections}, nil
}

// Timeline returns just the timeline events for evidence owned by ownerID.
func (s *Service) Timeline(ctx context.Context, evidenceID, ownerID uuid.UUID) ([]domain.TimelineEvent, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID); err != nil {
		return nil, err
	}
	return s.timeline.ListByEvidence(ctx, evidenceID)
}

// PII returns just the PII detections for evidence owned by ownerID.
func (s *Service) PII(ctx context.Context, evidenceID, ownerID uuid.UUID) ([]domain.PIIDetection, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID); err != nil {
		return nil, err
	}
	return s.piiRepo.ListByEvidence(ctx, evidenceID)
}
