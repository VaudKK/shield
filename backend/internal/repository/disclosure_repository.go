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

type DisclosureRepository struct {
	pool *pgxpool.Pool
}

func NewDisclosureRepository(pool *pgxpool.Pool) *DisclosureRepository {
	return &DisclosureRepository{pool: pool}
}

func (r *DisclosureRepository) Create(ctx context.Context, d domain.Disclosure, evidenceIDs []uuid.UUID) (*domain.Disclosure, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var out domain.Disclosure
	err = tx.QueryRow(ctx, `
		INSERT INTO disclosures (
			id, owner_id, title, include_timeline, include_summary, include_photos,
			remove_phone_numbers, remove_emails, remove_id_numbers, blur_faces, remove_metadata,
			storage_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, owner_id, title, include_timeline, include_summary, include_photos,
			remove_phone_numbers, remove_emails, remove_id_numbers, blur_faces, remove_metadata,
			storage_key, created_at
	`,
		d.ID, d.OwnerID, d.Title, d.IncludeTimeline, d.IncludeSummary, d.IncludePhotos,
		d.RemovePhoneNumbers, d.RemoveEmails, d.RemoveIDNumbers, d.BlurFaces, d.RemoveMetadata,
		d.StorageKey,
	).Scan(
		&out.ID, &out.OwnerID, &out.Title, &out.IncludeTimeline, &out.IncludeSummary, &out.IncludePhotos,
		&out.RemovePhoneNumbers, &out.RemoveEmails, &out.RemoveIDNumbers, &out.BlurFaces, &out.RemoveMetadata,
		&out.StorageKey, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert disclosure: %w", err)
	}

	for _, evidenceID := range evidenceIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO disclosure_evidence (disclosure_id, evidence_id) VALUES ($1, $2)
		`, out.ID, evidenceID); err != nil {
			return nil, fmt.Errorf("link disclosure evidence: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit disclosure: %w", err)
	}
	return &out, nil
}

func (r *DisclosureRepository) GetByIDForOwner(ctx context.Context, id, ownerID uuid.UUID) (*domain.Disclosure, error) {
	var out domain.Disclosure
	err := r.pool.QueryRow(ctx, `
		SELECT id, owner_id, title, include_timeline, include_summary, include_photos,
			remove_phone_numbers, remove_emails, remove_id_numbers, blur_faces, remove_metadata,
			storage_key, created_at
		FROM disclosures WHERE id = $1 AND owner_id = $2
	`, id, ownerID).Scan(
		&out.ID, &out.OwnerID, &out.Title, &out.IncludeTimeline, &out.IncludeSummary, &out.IncludePhotos,
		&out.RemovePhoneNumbers, &out.RemoveEmails, &out.RemoveIDNumbers, &out.BlurFaces, &out.RemoveMetadata,
		&out.StorageKey, &out.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select disclosure: %w", err)
	}
	return &out, nil
}

func (r *DisclosureRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]domain.Disclosure, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, owner_id, title, include_timeline, include_summary, include_photos,
			remove_phone_numbers, remove_emails, remove_id_numbers, blur_faces, remove_metadata,
			storage_key, created_at
		FROM disclosures WHERE owner_id = $1 ORDER BY created_at DESC
	`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list disclosures: %w", err)
	}
	defer rows.Close()

	var out []domain.Disclosure
	for rows.Next() {
		var d domain.Disclosure
		if err := rows.Scan(
			&d.ID, &d.OwnerID, &d.Title, &d.IncludeTimeline, &d.IncludeSummary, &d.IncludePhotos,
			&d.RemovePhoneNumbers, &d.RemoveEmails, &d.RemoveIDNumbers, &d.BlurFaces, &d.RemoveMetadata,
			&d.StorageKey, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan disclosure: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *DisclosureRepository) ListEvidenceIDs(ctx context.Context, disclosureID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT evidence_id FROM disclosure_evidence WHERE disclosure_id = $1
	`, disclosureID)
	if err != nil {
		return nil, fmt.Errorf("list disclosure evidence: %w", err)
	}
	defer rows.Close()

	var out []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan disclosure evidence id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
