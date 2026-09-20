package disclosure

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
)

type Detail struct {
	Disclosure  *domain.Disclosure
	EvidenceIDs []uuid.UUID
}

func (s *Service) Get(ctx context.Context, id, ownerID uuid.UUID) (*Detail, error) {
	d, err := s.disclosures.GetByIDForOwner(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}
	evidenceIDs, err := s.disclosures.ListEvidenceIDs(ctx, d.ID)
	if err != nil {
		return nil, fmt.Errorf("list disclosure evidence: %w", err)
	}
	return &Detail{Disclosure: d, EvidenceIDs: evidenceIDs}, nil
}

func (s *Service) List(ctx context.Context, ownerID uuid.UUID) ([]domain.Disclosure, error) {
	return s.disclosures.ListByOwner(ctx, ownerID)
}

// DownloadURL returns a fresh short-lived signed URL for an existing
// package — never a permanent public link.
func (s *Service) DownloadURL(ctx context.Context, id, ownerID uuid.UUID) (string, error) {
	d, err := s.disclosures.GetByIDForOwner(ctx, id, ownerID)
	if err != nil {
		return "", err
	}
	return s.storage.PresignGet(ctx, d.StorageKey, SignedURLTTL)
}
