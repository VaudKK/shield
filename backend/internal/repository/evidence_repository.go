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

type EvidenceRepository struct {
	pool *pgxpool.Pool
}

func NewEvidenceRepository(pool *pgxpool.Pool) *EvidenceRepository {
	return &EvidenceRepository{pool: pool}
}

func (r *EvidenceRepository) Create(ctx context.Context, ownerID uuid.UUID, title string, status domain.EvidenceStatus) (*domain.Evidence, error) {
	var e domain.Evidence
	err := r.pool.QueryRow(ctx, `
		INSERT INTO evidence (owner_id, title, status)
		VALUES ($1, $2, $3)
		RETURNING id, owner_id, title, status, created_at, updated_at, deleted_at
	`, ownerID, title, status).Scan(
		&e.ID, &e.OwnerID, &e.Title, &e.Status, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert evidence: %w", err)
	}
	return &e, nil
}

// GetByIDForOwner returns the evidence record only if it belongs to
// ownerID and has not been soft-deleted, so a lookup by a non-owner behaves
// identically to a lookup of a nonexistent ID.
func (r *EvidenceRepository) GetByIDForOwner(ctx context.Context, id, ownerID uuid.UUID) (*domain.Evidence, error) {
	var e domain.Evidence
	err := r.pool.QueryRow(ctx, `
		SELECT id, owner_id, title, status, created_at, updated_at, deleted_at
		FROM evidence
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL
	`, id, ownerID).Scan(
		&e.ID, &e.OwnerID, &e.Title, &e.Status, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select evidence: %w", err)
	}
	return &e, nil
}

func (r *EvidenceRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, limit int) ([]domain.Evidence, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, title, status, created_at, updated_at, deleted_at
		FROM evidence
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2
	`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()

	var out []domain.Evidence
	for rows.Next() {
		var e domain.Evidence
		if err := rows.Scan(&e.ID, &e.OwnerID, &e.Title, &e.Status, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt); err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *EvidenceRepository) SoftDelete(ctx context.Context, id, ownerID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE evidence SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL
	`, id, ownerID)
	if err != nil {
		return fmt.Errorf("soft delete evidence: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
