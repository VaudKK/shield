// Package auth implements Shield's authentication business logic: user
// registration, credential verification, and session issuance/validation.
// It has no knowledge of HTTP — cookies and status codes are the httpapi
// layer's job.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/VaudKK/shield/backend/internal/repository"
	"github.com/VaudKK/shield/backend/internal/security"
	"github.com/google/uuid"
)

const SessionDuration = 7 * 24 * time.Hour

var (
	ErrEmailTaken           = errors.New("email already registered")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrInvalidSession       = errors.New("invalid or expired session")
	ErrInvalidVaultRecovery = errors.New("invalid vault ID or recovery key")
)

type Service struct {
	users    *repository.UserRepository
	sessions *repository.SessionRepository
}

func NewService(users *repository.UserRepository, sessions *repository.SessionRepository) *Service {
	return &Service{users: users, sessions: sessions}
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (s *Service) Register(ctx context.Context, email, password, displayName string) (*domain.User, error) {
	email = NormalizeEmail(email)

	exists, err := s.users.EmailExists(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}

	return s.users.Create(ctx, email, hash, strings.TrimSpace(displayName))
}

const maxVaultIDAttempts = 5

// CreateVault creates a new anonymous vault: a user row with no email,
// identified by a freshly generated public vault ID and secured by a
// freshly generated high-entropy recovery key. The raw recovery key is
// returned exactly once — only its bcrypt hash is persisted, exactly like
// a password. Callers must not log rawRecoveryKey.
func (s *Service) CreateVault(ctx context.Context) (user *domain.User, rawRecoveryKey string, err error) {
	rawRecoveryKey, err = security.GenerateRecoveryKey()
	if err != nil {
		return nil, "", err
	}
	hash, err := security.HashPassword(rawRecoveryKey)
	if err != nil {
		return nil, "", err
	}

	for range maxVaultIDAttempts {
		vaultID, genErr := security.GenerateVaultID()
		if genErr != nil {
			return nil, "", genErr
		}

		taken, existsErr := s.users.VaultIDExists(ctx, vaultID)
		if existsErr != nil {
			return nil, "", existsErr
		}
		if taken {
			continue
		}

		user, err = s.users.CreateVault(ctx, vaultID, hash)
		if err != nil {
			return nil, "", err
		}
		return user, rawRecoveryKey, nil
	}

	return nil, "", fmt.Errorf("could not generate a unique vault ID after %d attempts", maxVaultIDAttempts)
}

// RecoverVault authenticates a vault by its public ID and secret recovery
// key. Failure modes (unknown vault ID vs. wrong key) are indistinguishable
// to the caller, and take roughly the same time either way.
func (s *Service) RecoverVault(ctx context.Context, vaultID, recoveryKey string) (*domain.User, error) {
	user, err := s.users.GetByVaultID(ctx, strings.TrimSpace(vaultID))
	if errors.Is(err, domain.ErrNotFound) {
		security.VerifyPassword("$2a$12$00000000000000000000000000000000000000000000000000", recoveryKey)
		return nil, ErrInvalidVaultRecovery
	}
	if err != nil {
		return nil, err
	}

	if !security.VerifyPassword(user.PasswordHash, recoveryKey) {
		return nil, ErrInvalidVaultRecovery
	}

	return user, nil
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.users.GetByEmail(ctx, NormalizeEmail(email))
	if errors.Is(err, domain.ErrNotFound) {
		// Hash a dummy value so lookups for unknown vs known emails take
		// roughly the same amount of time.
		security.VerifyPassword("$2a$12$00000000000000000000000000000000000000000000000000", password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if !security.VerifyPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// IssueSession creates a new session for the user and returns the raw token
// to be stored in the client's cookie, along with its expiry.
func (s *Service) IssueSession(ctx context.Context, userID uuid.UUID) (rawToken string, expiresAt time.Time, err error) {
	raw, hash, err := security.GenerateToken()
	if err != nil {
		return "", time.Time{}, err
	}

	expiresAt = time.Now().Add(SessionDuration)
	if _, err := s.sessions.Create(ctx, userID, hash, expiresAt); err != nil {
		return "", time.Time{}, err
	}

	return raw, expiresAt, nil
}

// SessionUser resolves the raw session token from a request cookie to the
// user it belongs to, or ErrInvalidSession if it is missing, expired, or
// unknown.
func (s *Service) SessionUser(ctx context.Context, rawToken string) (*domain.User, error) {
	if rawToken == "" {
		return nil, ErrInvalidSession
	}

	session, err := s.sessions.GetByTokenHash(ctx, security.HashToken(rawToken))
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		_ = s.sessions.DeleteByTokenHash(ctx, session.TokenHash)
		return nil, ErrInvalidSession
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.sessions.DeleteByTokenHash(ctx, security.HashToken(rawToken))
}
