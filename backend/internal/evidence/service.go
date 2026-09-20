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
	evidence *repository.EvidenceRepository
	files    *repository.EvidenceFileRepository
	audit    *repository.AuditRepository
	storage  storage.Storage
}

func NewService(
	evidenceRepo *repository.EvidenceRepository,
	filesRepo *repository.EvidenceFileRepository,
	auditRepo *repository.AuditRepository,
	store storage.Storage,
) *Service {
	return &Service{evidence: evidenceRepo, files: filesRepo, audit: auditRepo, storage: store}
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
// unmodified; only a SHA-256 hash and metadata are computed locally.
func (s *Service) Upload(ctx context.Context, ownerID uuid.UUID, title string, file multipart.File, filename string) (*UploadResult, error) {
	mimeType, _, err := sniffAndValidate(file)
	if err != nil {
		return nil, err
	}

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

	if err := s.storage.PutOriginal(ctx, storageKey, tee, mimeType); err != nil {
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

	return &UploadResult{Evidence: ev, File: ef}, nil
}

func extensionFor(filename string) string {
	if i := strings.LastIndex(filename, "."); i != -1 {
		return filename[i:]
	}
	return ""
}
