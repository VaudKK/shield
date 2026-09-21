package evidence

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
)

type Detail struct {
	Evidence    *domain.Evidence
	Files       []domain.EvidenceFile
	OriginalURL string               // short-lived signed URL for the original file, if any
	FileURLs    map[uuid.UUID]string // short-lived signed URL per file ID, for original and redacted kinds
}

// Get loads an evidence record owned by ownerID, presigns short-lived URLs
// for its original and any redacted files, and records an EVIDENCE_VIEWED
// audit event. It returns domain.ErrNotFound if the evidence doesn't exist
// or isn't owned by ownerID — the two cases are indistinguishable to the
// caller.
func (s *Service) Get(ctx context.Context, id, ownerID uuid.UUID) (*Detail, error) {
	ev, err := s.evidence.GetByIDForOwner(ctx, id, ownerID)
	if err != nil {
		return nil, err
	}

	files, err := s.files.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}

	detail := &Detail{Evidence: ev, Files: files, FileURLs: make(map[uuid.UUID]string)}

	for _, f := range files {
		if f.Kind != domain.EvidenceFileKindOriginal && f.Kind != domain.EvidenceFileKindRedacted {
			continue
		}
		url, err := s.storage.PresignGet(ctx, f.StorageKey, SignedURLTTL)
		if err != nil {
			return nil, fmt.Errorf("presign %s file: %w", f.Kind, err)
		}
		detail.FileURLs[f.ID] = url
		if f.Kind == domain.EvidenceFileKindOriginal && detail.OriginalURL == "" {
			detail.OriginalURL = url
		}
	}

	actor := ownerID
	if err := s.audit.Record(ctx, ev.ID, domain.AuditEventEvidenceViewed, &actor, nil); err != nil {
		return nil, fmt.Errorf("record view audit event: %w", err)
	}

	return detail, nil
}

func (s *Service) List(ctx context.Context, ownerID uuid.UUID) ([]domain.Evidence, error) {
	return s.evidence.ListByOwner(ctx, ownerID, 100)
}

func (s *Service) Delete(ctx context.Context, id, ownerID uuid.UUID) error {
	if err := s.evidence.SoftDelete(ctx, id, ownerID); err != nil {
		return err
	}
	actor := ownerID
	return s.audit.Record(ctx, id, domain.AuditEventEvidenceDeleted, &actor, nil)
}

// AuditTrail returns the append-only history for a piece of evidence owned
// by ownerID.
func (s *Service) AuditTrail(ctx context.Context, id, ownerID uuid.UUID) ([]domain.AuditEvent, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, id, ownerID); err != nil {
		return nil, err
	}
	return s.audit.ListByEvidence(ctx, id)
}

// Moderation returns the most recent content-safety scan for a piece of
// evidence owned by ownerID, or domain.ErrNotFound if it was never scanned
// (e.g. a PDF, or content-safety scanning wasn't configured).
func (s *Service) Moderation(ctx context.Context, id, ownerID uuid.UUID) (*domain.ModerationResult, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, id, ownerID); err != nil {
		return nil, err
	}
	return s.moderation.GetLatestByEvidence(ctx, id)
}
