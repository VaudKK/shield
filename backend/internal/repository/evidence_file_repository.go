package repository

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EvidenceFileRepository struct {
	pool *pgxpool.Pool
}

func NewEvidenceFileRepository(pool *pgxpool.Pool) *EvidenceFileRepository {
	return &EvidenceFileRepository{pool: pool}
}

func (r *EvidenceFileRepository) Create(ctx context.Context, f domain.EvidenceFile) (*domain.EvidenceFile, error) {
	var out domain.EvidenceFile
	err := r.pool.QueryRow(ctx, `
		INSERT INTO evidence_files (evidence_id, kind, original_filename, mime_type, size_bytes, sha256, storage_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, evidence_id, kind, original_filename, mime_type, size_bytes, sha256, storage_key, created_at
	`, f.EvidenceID, f.Kind, f.OriginalFilename, f.MimeType, f.SizeBytes, f.SHA256, f.StorageKey).Scan(
		&out.ID, &out.EvidenceID, &out.Kind, &out.OriginalFilename, &out.MimeType,
		&out.SizeBytes, &out.SHA256, &out.StorageKey, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert evidence file: %w", err)
	}
	return &out, nil
}

func (r *EvidenceFileRepository) ListByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]domain.EvidenceFile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evidence_id, kind, original_filename, mime_type, size_bytes, sha256, storage_key, created_at
		FROM evidence_files
		WHERE evidence_id = $1
		ORDER BY created_at ASC
	`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list evidence files: %w", err)
	}
	defer rows.Close()

	var out []domain.EvidenceFile
	for rows.Next() {
		var f domain.EvidenceFile
		if err := rows.Scan(&f.ID, &f.EvidenceID, &f.Kind, &f.OriginalFilename, &f.MimeType,
			&f.SizeBytes, &f.SHA256, &f.StorageKey, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan evidence file: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
