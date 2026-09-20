package repository

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RedactionRepository struct {
	pool *pgxpool.Pool
}

func NewRedactionRepository(pool *pgxpool.Pool) *RedactionRepository {
	return &RedactionRepository{pool: pool}
}

func (r *RedactionRepository) Create(ctx context.Context, red domain.Redaction) (*domain.Redaction, error) {
	var out domain.Redaction
	err := r.pool.QueryRow(ctx, `
		INSERT INTO redactions (evidence_id, evidence_file_id, pii_detection_id, applied)
		VALUES ($1, $2, $3, $4)
		RETURNING id, evidence_id, evidence_file_id, pii_detection_id, applied, created_at
	`, red.EvidenceID, red.EvidenceFileID, red.PIIDetectionID, red.Applied).Scan(
		&out.ID, &out.EvidenceID, &out.EvidenceFileID, &out.PIIDetectionID, &out.Applied, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert redaction: %w", err)
	}
	return &out, nil
}

func (r *RedactionRepository) ListByEvidenceFile(ctx context.Context, evidenceFileID uuid.UUID) ([]domain.Redaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evidence_id, evidence_file_id, pii_detection_id, applied, created_at
		FROM redactions
		WHERE evidence_file_id = $1
		ORDER BY created_at ASC
	`, evidenceFileID)
	if err != nil {
		return nil, fmt.Errorf("list redactions: %w", err)
	}
	defer rows.Close()

	var out []domain.Redaction
	for rows.Next() {
		var red domain.Redaction
		if err := rows.Scan(&red.ID, &red.EvidenceID, &red.EvidenceFileID, &red.PIIDetectionID, &red.Applied, &red.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan redaction: %w", err)
		}
		out = append(out, red)
	}
	return out, rows.Err()
}
