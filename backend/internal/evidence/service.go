package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/VaudKK/shield/backend/internal/contentsafety"
	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/storage"
	"github.com/google/uuid"
)

// MaxUploadBytes bounds the whole multipart request body, not just the file
// part, leaving headroom for form fields and multipart boundaries.
const MaxUploadBytes = 55 << 20 // 55 MiB

// SignedURLTTL is how long a presigned link to an original file stays
// valid. Original evidence is never exposed through a permanent public URL.
const SignedURLTTL = 5 * time.Minute

type Service struct {
	evidence   *repository.EvidenceRepository
	files      *repository.EvidenceFileRepository
	audit      *repository.AuditRepository
	moderation *repository.ModerationRepository
	storage    storage.Storage
	moderator  contentsafety.ModerationService // nil disables content-safety scanning
}

func NewService(
	evidenceRepo *repository.EvidenceRepository,
	filesRepo *repository.EvidenceFileRepository,
	auditRepo *repository.AuditRepository,
	moderationRepo *repository.ModerationRepository,
	store storage.Storage,
	moderator contentsafety.ModerationService,
) *Service {
	return &Service{
		evidence:   evidenceRepo,
		files:      filesRepo,
		audit:      auditRepo,
		moderation: moderationRepo,
		storage:    store,
		moderator:  moderator,
	}
}

type UploadResult struct {
	Evidence *domain.Evidence
	File     *domain.EvidenceFile
}

// countingWriter tracks how many bytes were actually streamed, so file size
// is derived from what was hashed and stored rather than trusted from the
// client-supplied multipart header.
type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// Upload validates, hashes, and stores one file as new evidence owned by
// ownerID. The original bytes are streamed straight to object storage
// unmodified; only a SHA-256 hash and metadata are computed locally. Images
// are then scanned for sensitive content directly from the stored original
// (see scanContentSafety) — nothing about the original file itself is ever
// touched by that scan.
func (s *Service) Upload(ctx context.Context, ownerID uuid.UUID, title string, file multipart.File, filename string) (*UploadResult, error) {
	mimeType, _, err := sniffAndValidate(file)
	if err != nil {
		return nil, err
	}
	isImage := strings.HasPrefix(mimeType, "image/")

	if title == "" {
		title = filename
	}

	ev, err := s.evidence.Create(ctx, ownerID, title, domain.EvidenceStatusQuarantined)
	if err != nil {
		return nil, fmt.Errorf("create evidence record: %w", err)
	}

	fileID := uuid.New()
	storageKey := fmt.Sprintf("originals/%s/%s%s", ev.ID, fileID, extensionFor(filename))

	hasher := sha256.New()
	counter := &countingWriter{}
	tee := io.TeeReader(file, io.MultiWriter(hasher, counter))

	if err := s.storage.Put(ctx, storageKey, tee, mimeType); err != nil {
		return nil, fmt.Errorf("store original: %w", err)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))

	ef, err := s.files.Create(ctx, domain.EvidenceFile{
		EvidenceID:       ev.ID,
		Kind:             domain.EvidenceFileKindOriginal,
		OriginalFilename: filename,
		MimeType:         mimeType,
		SizeBytes:        counter.n,
		SHA256:           sum,
		StorageKey:       storageKey,
	})
	if err != nil {
		return nil, fmt.Errorf("record evidence file: %w", err)
	}

	actor := ownerID
	if err := s.audit.Record(ctx, ev.ID, domain.AuditEventEvidenceUploaded, &actor, map[string]any{
		"filename": filename,
	}); err != nil {
		return nil, fmt.Errorf("record upload audit event: %w", err)
	}
	if err := s.audit.Record(ctx, ev.ID, domain.AuditEventHashCreated, &actor, map[string]any{
		"sha256": sum,
	}); err != nil {
		return nil, fmt.Errorf("record hash audit event: %w", err)
	}

	if isImage {
		ev.Status = s.scanContentSafety(ctx, ev.ID, storageKey)
	} else {
		// NudeNet only classifies images. PDFs skip straight to "safe" for
		// content-safety purposes (nudity detection genuinely doesn't apply
		// to a document), which is recorded explicitly so the audit trail
		// never implies a scan happened when it didn't.
		ev.Status = domain.EvidenceStatusSafe
		if err := s.evidence.UpdateStatus(ctx, ev.ID, ev.Status); err != nil {
			return nil, fmt.Errorf("update evidence status: %w", err)
		}
		_ = s.audit.Record(ctx, ev.ID, domain.AuditEventContentSafetyChecked, nil, map[string]any{
			"status": string(ev.Status),
			"note":   "content safety scanning does not apply to non-image file types",
		})
	}

	return &UploadResult{Evidence: ev, File: ef}, nil
}

// scanContentSafety runs the stored original through the configured
// ModerationService (if any), persists the result, updates the evidence
// status, and records an audit event either way. It never returns an
// error to the caller: a scan failure degrades to "needs review" rather
// than failing the whole upload, since the evidence is already safely
// stored. The scan result is a risk signal only — this function never
// rejects or deletes evidence, regardless of what it finds.
func (s *Service) scanContentSafety(ctx context.Context, evidenceID uuid.UUID, storageKey string) domain.EvidenceStatus {
	if s.moderator == nil {
		// No moderation service configured: leave evidence quarantined
		// rather than claiming a safety verdict nothing actually checked.
		return domain.EvidenceStatusQuarantined
	}

	result, err := s.moderator.Scan(ctx, storageKey)

	metadata := map[string]any{}
	if err != nil {
		metadata["error"] = err.Error()
		metadata["status"] = string(domain.EvidenceStatusReview)
		_ = s.audit.Record(ctx, evidenceID, domain.AuditEventContentSafetyChecked, nil, metadata)
		if updateErr := s.evidence.UpdateStatus(ctx, evidenceID, domain.EvidenceStatusReview); updateErr != nil {
			s.recordUpdateError(ctx, evidenceID, updateErr)
		}
		return domain.EvidenceStatusReview
	}

	status := domain.EvidenceStatus(result.Status)
	metadata["status"] = string(status)
	metadata["confidence"] = result.Confidence
	metadata["labels"] = result.Labels

	boxes := make([]domain.BoundingBox, len(result.BoundingBox))
	for i, b := range result.BoundingBox {
		boxes[i] = domain.BoundingBox{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
	}
	if _, err := s.moderation.Create(ctx, domain.ModerationResult{
		EvidenceID:  evidenceID,
		Status:      status,
		Confidence:  result.Confidence,
		Labels:      result.Labels,
		BoundingBox: boxes,
	}); err != nil {
		metadata["persist_error"] = err.Error()
	}

	if updateErr := s.evidence.UpdateStatus(ctx, evidenceID, status); updateErr != nil {
		s.recordUpdateError(ctx, evidenceID, updateErr)
	}
	_ = s.audit.Record(ctx, evidenceID, domain.AuditEventContentSafetyChecked, nil, metadata)

	return status
}

func (s *Service) recordUpdateError(ctx context.Context, evidenceID uuid.UUID, updateErr error) {
	_ = s.audit.Record(ctx, evidenceID, domain.AuditEventContentSafetyChecked, nil, map[string]any{
		"update_error": updateErr.Error(),
	})
}

func extensionFor(filename string) string {
	if i := strings.LastIndex(filename, "."); i != -1 {
		return filename[i:]
	}
	return ""
}
