// Package repository contains PostgreSQL-backed persistence for domain
// types. Repositories return domain.ErrNotFound rather than leaking
// pgx-specific errors to callers.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userColumns = "id, email, vault_id, password_hash, display_name, created_at, updated_at"

func scanUser(row interface{ Scan(...any) error }, u *domain.User) error {
	return row.Scan(&u.ID, &u.Email, &u.VaultID, &u.PasswordHash, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash, displayName string) (*domain.User, error) {
	var u domain.User
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns, email, passwordHash, displayName)
	if err := scanUser(row, &u); err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return &u, nil
}

// CreateVault inserts an anonymous vault user: no email, identified by
// vaultID, authenticated by a hashed recovery key (passed in recoveryHash,
// stored in the same password_hash column a password would use).
func (r *UserRepository) CreateVault(ctx context.Context, vaultID, recoveryHash string) (*domain.User, error) {
	var u domain.User
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (vault_id, password_hash, display_name)
		VALUES ($1, $2, 'Anonymous Vault')
		RETURNING `+userColumns, vaultID, recoveryHash)
	if err := scanUser(row, &u); err != nil {
		return nil, fmt.Errorf("insert vault user: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	if err := scanUser(row, &u); errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByVaultID(ctx context.Context, vaultID string) (*domain.User, error) {
	var u domain.User
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE vault_id = $1`, vaultID)
	if err := scanUser(row, &u); errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("select user by vault id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	if err := scanUser(row, &u); errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	} else if err != nil {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return &u, nil
}

// EmailExists reports whether a user with the given email already exists,
// without leaking any other row data (used for the register uniqueness check).
func (r *UserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return exists, nil
}

// VaultIDExists reports whether a vault ID is already in use — checked
// before insert so a random collision can be retried with a fresh ID
// rather than failing the request.
func (r *UserRepository) VaultIDExists(ctx context.Context, vaultID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE vault_id = $1)`, vaultID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check vault id exists: %w", err)
	}
	return exists, nil
}
