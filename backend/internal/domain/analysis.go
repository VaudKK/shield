package domain

import (
	"time"

	"github.com/google/uuid"
)

type Gap struct {
	Description string `json:"description"`
	Confidence  string `json:"confidence"`
}

type Analysis struct {
	ID         uuid.UUID
	EvidenceID uuid.UUID
	OCRText    string
	Summary    string
	Gaps       []Gap
	Model      string
	AnalyzedAt time.Time
}

type TimelineSource string

const (
	TimelineSourceAI   TimelineSource = "ai"
	TimelineSourceUser TimelineSource = "user"
)

type TimelineEvent struct {
	ID          uuid.UUID
	EvidenceID  uuid.UUID
	EventDate   string
	Description string
	Source      TimelineSource
	CreatedAt   time.Time
}

type PIIDetectionMethod string

const (
	PIIDetectionMethodRegex  PIIDetectionMethod = "regex"
	PIIDetectionMethodAI     PIIDetectionMethod = "ai"
	PIIDetectionMethodManual PIIDetectionMethod = "manual"
)

type PIIStatus string

const (
	PIIStatusDetected PIIStatus = "detected"
	PIIStatusAccepted PIIStatus = "accepted"
	PIIStatusRejected PIIStatus = "rejected"
)

type PIIDetection struct {
	ID              uuid.UUID
	EvidenceID      uuid.UUID
	Type            string
	Value           string
	Location        string
	DetectionMethod PIIDetectionMethod
	Status          PIIStatus
	CreatedAt       time.Time
}

type Redaction struct {
	ID             uuid.UUID
	EvidenceID     uuid.UUID
	EvidenceFileID uuid.UUID
	PIIDetectionID uuid.UUID
	Applied        bool
	CreatedAt      time.Time
}
