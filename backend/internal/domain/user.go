package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID uuid.UUID
	// Exactly one of Email or VaultID is set: a user is either an
	// email/password account or an anonymous vault, never both.
	Email        *string
	VaultID      *string
	PasswordHash string // holds the bcrypt hash of a password OR a recovery key
	DisplayName  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) IsVault() bool {
	return u.VaultID != nil
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}
