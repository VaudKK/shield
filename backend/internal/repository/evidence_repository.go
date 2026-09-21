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
		SELECT e.id, e.owner_id, e.title, e.status, e.created_at, e.updated_at, e.deleted_at,
		       EXISTS (SELECT 1 FROM evidence_analysis a WHERE a.evidence_id = e.id)
		FROM evidence e
		WHERE e.id = $1 AND e.owner_id = $2 AND e.deleted_at IS NULL
	`, id, ownerID).Scan(
		&e.ID, &e.OwnerID, &e.Title, &e.Status, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt, &e.Analyzed,
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
		SELECT e.id, e.owner_id, e.title, e.status, e.created_at, e.updated_at, e.deleted_at,
		       EXISTS (SELECT 1 FROM evidence_analysis a WHERE a.evidence_id = e.id)
		FROM evidence e
		WHERE e.owner_id = $1 AND e.deleted_at IS NULL
		ORDER BY e.created_at DESC
		LIMIT $2
	`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	defer rows.Close()

	var out []domain.Evidence
	for rows.Next() {
		var e domain.Evidence
		if err := rows.Scan(&e.ID, &e.OwnerID, &e.Title, &e.Status, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt, &e.Analyzed); err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateStatus is not scoped to an owner: it is called by internal
// pipeline steps (content safety, future AI analysis) acting on behalf of
// the system, not a specific user's request.
func (r *EvidenceRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.EvidenceStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE evidence SET status = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, status)
	if err != nil {
		return fmt.Errorf("update evidence status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
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
