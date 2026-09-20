package repository

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PIIRepository struct {
	pool *pgxpool.Pool
}

func NewPIIRepository(pool *pgxpool.Pool) *PIIRepository {
	return &PIIRepository{pool: pool}
}

// ReplaceForEvidence swaps out all PII detections for this evidence with a
// fresh set, so re-running analysis doesn't accumulate duplicates. Any
// review decisions (accepted/rejected) made by the user are necessarily
// reset — the UI should prompt for review again after re-analysis.
func (r *PIIRepository) ReplaceForEvidence(ctx context.Context, evidenceID uuid.UUID, detections []domain.PIIDetection) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM pii_detections WHERE evidence_id = $1`, evidenceID,
	); err != nil {
		return fmt.Errorf("delete old pii detections: %w", err)
	}

	for _, d := range detections {
		if _, err := tx.Exec(ctx, `
			INSERT INTO pii_detections (evidence_id, pii_type, value, location, detection_method)
			VALUES ($1, $2, $3, $4, $5)
		`, evidenceID, d.Type, d.Value, d.Location, d.DetectionMethod); err != nil {
			return fmt.Errorf("insert pii detection: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PIIRepository) ListByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]domain.PIIDetection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evidence_id, pii_type, value, location, detection_method, status, created_at
		FROM pii_detections
		WHERE evidence_id = $1
		ORDER BY created_at ASC
	`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list pii detections: %w", err)
	}
	defer rows.Close()

	var out []domain.PIIDetection
	for rows.Next() {
		var d domain.PIIDetection
		if err := rows.Scan(&d.ID, &d.EvidenceID, &d.Type, &d.Value, &d.Location, &d.DetectionMethod, &d.Status, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pii detection: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
