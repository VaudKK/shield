package domain

import (
	"time"

	"github.com/google/uuid"
)

type Disclosure struct {
	ID      uuid.UUID
	OwnerID uuid.UUID
	Title   string

	IncludeTimeline bool
	IncludeSummary  bool
	IncludePhotos   bool

	RemovePhoneNumbers bool
	RemoveEmails       bool
	RemoveIDNumbers    bool
	BlurFaces          bool
	RemoveMetadata     bool

	StorageKey string
	CreatedAt  time.Time
}
