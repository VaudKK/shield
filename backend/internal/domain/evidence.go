package domain

import (
	"time"

	"github.com/google/uuid"
)

type EvidenceStatus string

const (
	EvidenceStatusQuarantined EvidenceStatus = "quarantined"
	EvidenceStatusSafe        EvidenceStatus = "safe"
	EvidenceStatusReview      EvidenceStatus = "review"
	EvidenceStatusSensitive   EvidenceStatus = "sensitive"
	EvidenceStatusRejected    EvidenceStatus = "rejected"
)

type Evidence struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Title     string
	Status    EvidenceStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type EvidenceFileKind string

const (
	EvidenceFileKindOriginal EvidenceFileKind = "original"
	EvidenceFileKindPreview  EvidenceFileKind = "preview"
	EvidenceFileKindRedacted EvidenceFileKind = "redacted"
	EvidenceFileKindExport   EvidenceFileKind = "export"
)

type EvidenceFile struct {
	ID               uuid.UUID
	EvidenceID       uuid.UUID
	Kind             EvidenceFileKind
	OriginalFilename string
	MimeType         string
	SizeBytes        int64
	SHA256           string
	StorageKey       string
	CreatedAt        time.Time
}

const (
	AuditEventEvidenceUploaded     = "EVIDENCE_UPLOADED"
	AuditEventHashCreated          = "HASH_CREATED"
	AuditEventEvidenceViewed       = "EVIDENCE_VIEWED"
	AuditEventEvidenceDeleted      = "EVIDENCE_DELETED"
	AuditEventContentSafetyChecked = "CONTENT_SAFETY_CHECKED"
	AuditEventOCRCompleted         = "OCR_COMPLETED"
	AuditEventAIAnalysisCompleted  = "AI_ANALYSIS_COMPLETED"
	AuditEventPIIDetected          = "PII_DETECTED"
	AuditEventRedactionCreated     = "REDACTION_CREATED"
	AuditEventExportCreated        = "EXPORT_CREATED"
)

type AuditEvent struct {
	ID         uuid.UUID
	EvidenceID uuid.UUID
	EventType  string
	ActorID    *uuid.UUID
	Metadata   map[string]any
	CreatedAt  time.Time
}
