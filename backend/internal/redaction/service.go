// Package redaction implements the user-driven PII review and redaction
// workflow: reviewing detected PII, adding manual entries, and producing a
// derived redacted copy of the evidence. The original file is never
// modified — a redaction always creates a new evidence_files row.
package redaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/ocr"
	"github.com/VaudKK/shield/backend/internal/pdfredact"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
	"github.com/google/uuid"
)

// SignedURLTTL is how long a presigned link to a redacted file stays
// valid. Matches evidence.SignedURLTTL; kept as its own constant so this
// package doesn't need to import evidence for one value.
const SignedURLTTL = 5 * time.Minute

// wordBoxer is satisfied by ocr.CompositeService. Declared locally so this
// package depends only on the capability it needs.
type wordBoxer interface {
	ExtractWordBoxes(ctx context.Context, data []byte, mimeType string) ([]ocr.BoxedWord, error)
}

type Service struct {
	evidence      *repository.EvidenceRepository
	files         *repository.EvidenceFileRepository
	audit         *repository.AuditRepository
	piiRepo       *repository.PIIRepository
	redactionRepo *repository.RedactionRepository
	analysisRepo  *repository.AnalysisRepository
	storage       storage.Storage
	ocr           wordBoxer
	pdfRedactor   pdfredact.Service // nil falls back to a redacted text transcript
}

func NewService(
	evidenceRepo *repository.EvidenceRepository,
	filesRepo *repository.EvidenceFileRepository,
	auditRepo *repository.AuditRepository,
	piiRepo *repository.PIIRepository,
	redactionRepo *repository.RedactionRepository,
	analysisRepo *repository.AnalysisRepository,
	store storage.Storage,
	ocrService wordBoxer,
	pdfRedactor pdfredact.Service,
) *Service {
	return &Service{
		evidence:      evidenceRepo,
		files:         filesRepo,
		audit:         auditRepo,
		piiRepo:       piiRepo,
		redactionRepo: redactionRepo,
		analysisRepo:  analysisRepo,
		storage:       store,
		ocr:           ocrService,
		pdfRedactor:   pdfRedactor,
	}
}

// ReviewPII records the user's accept/reject decision for one detection.
func (s *Service) ReviewPII(ctx context.Context, evidenceID, ownerID, piiID uuid.UUID, status domain.PIIStatus) (*domain.PIIDetection, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID); err != nil {
		return nil, err
	}
	if err := s.piiRepo.UpdateStatus(ctx, piiID, evidenceID, status); err != nil {
		return nil, err
	}
	return s.piiRepo.GetByID(ctx, piiID)
}

// AddManualPII adds a user-supplied PII item, defaulting to accepted since
// the user is adding it specifically so it gets redacted.
func (s *Service) AddManualPII(ctx context.Context, evidenceID, ownerID uuid.UUID, piiType, value, location string) (*domain.PIIDetection, error) {
	if _, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID); err != nil {
		return nil, err
	}
	return s.piiRepo.Create(ctx, domain.PIIDetection{
		EvidenceID:      evidenceID,
		Type:            piiType,
		Value:           value,
		Location:        location,
		DetectionMethod: domain.PIIDetectionMethodManual,
		Status:          domain.PIIStatusAccepted,
	})
}

type RedactResult struct {
	File       *domain.EvidenceFile
	FileURL    string // short-lived signed URL for the new redacted file
	Redactions []domain.Redaction
}

var ErrNoAcceptedPII = fmt.Errorf("no accepted PII to redact")

// Redact produces a new derived file with every accepted PII value covered
// (images: real pixel redaction over its located position) or replaced
// (non-image fallback: a redacted text transcript). The original is never
// modified. Values that can't be located are recorded as not applied
// rather than silently dropped or failing the whole request.
func (s *Service) Redact(ctx context.Context, evidenceID, ownerID uuid.UUID) (*RedactResult, error) {
	ev, err := s.evidence.GetByIDForOwner(ctx, evidenceID, ownerID)
	if err != nil {
		return nil, err
	}

	files, err := s.files.ListByEvidence(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}
	var original *domain.EvidenceFile
	for i := range files {
		if files[i].Kind == domain.EvidenceFileKindOriginal {
			original = &files[i]
			break
		}
	}
	if original == nil {
		return nil, fmt.Errorf("evidence has no original file")
	}

	accepted, err := s.piiRepo.ListAcceptedByEvidence(ctx, ev.ID)
	if err != nil {
		return nil, fmt.Errorf("list accepted pii: %w", err)
	}
	if len(accepted) == 0 {
		return nil, ErrNoAcceptedPII
	}

	data, err := s.storage.GetObject(ctx, original.StorageKey)
	if err != nil {
		return nil, fmt.Errorf("fetch original for redaction: %w", err)
	}

	isImage := strings.HasPrefix(original.MimeType, "image/")

	var redactedBytes []byte
	var redactedMime, redactedExt, method string
	applied := make(map[uuid.UUID]bool, len(accepted))

	if isImage {
		redactedBytes, redactedMime, redactedExt, err = s.redactImageFile(ctx, data, original.MimeType, accepted, applied)
		if err != nil {
			return nil, err
		}
		method = "image_pixel"
	} else {
		redactedBytes, redactedMime, redactedExt, method, err = s.redactPDFFile(ctx, ev.ID, data, accepted, applied)
		if err != nil {
			return nil, err
		}
	}

	fileID := uuid.New()
	storageKey := fmt.Sprintf("redacted/%s/%s%s", ev.ID, fileID, redactedExt)
	if err := s.storage.Put(ctx, storageKey, bytesReader(redactedBytes), redactedMime); err != nil {
		return nil, fmt.Errorf("store redacted file: %w", err)
	}

	sum := sha256.Sum256(redactedBytes)

	ef, err := s.files.Create(ctx, domain.EvidenceFile{
		EvidenceID:       ev.ID,
		Kind:             domain.EvidenceFileKindRedacted,
		OriginalFilename: redactedFilename(original.OriginalFilename, redactedExt),
		MimeType:         redactedMime,
		SizeBytes:        int64(len(redactedBytes)),
		SHA256:           hex.EncodeToString(sum[:]),
		StorageKey:       storageKey,
	})
	if err != nil {
		return nil, fmt.Errorf("record redacted file: %w", err)
	}

	fileURL, err := s.storage.PresignGet(ctx, storageKey, SignedURLTTL)
	if err != nil {
		return nil, fmt.Errorf("presign redacted file: %w", err)
	}

	redactions := make([]domain.Redaction, 0, len(accepted))
	appliedCount := 0
	for _, item := range accepted {
		wasApplied := applied[item.ID]
		if wasApplied {
			appliedCount++
		}
		red, err := s.redactionRepo.Create(ctx, domain.Redaction{
			EvidenceID:     ev.ID,
			EvidenceFileID: ef.ID,
			PIIDetectionID: item.ID,
			Applied:        wasApplied,
		})
		if err != nil {
			return nil, fmt.Errorf("record redaction: %w", err)
		}
		redactions = append(redactions, *red)
	}

	actor := ownerID
	_ = s.audit.Record(ctx, ev.ID, domain.AuditEventRedactionCreated, &actor, map[string]any{
		"redacted_file_id": ef.ID.String(),
		"requested_count":  len(accepted),
		"applied_count":    appliedCount,
		"method":           method,
	})

	return &RedactResult{File: ef, FileURL: fileURL, Redactions: redactions}, nil
}
