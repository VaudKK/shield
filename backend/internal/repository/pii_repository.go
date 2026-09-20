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

// Create adds a single PII detection (used for user-added manual entries).
func (r *PIIRepository) Create(ctx context.Context, d domain.PIIDetection) (*domain.PIIDetection, error) {
	var out domain.PIIDetection
	err := r.pool.QueryRow(ctx, `
		INSERT INTO pii_detections (evidence_id, pii_type, value, location, detection_method, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, evidence_id, pii_type, value, location, detection_method, status, created_at
	`, d.EvidenceID, d.Type, d.Value, d.Location, d.DetectionMethod, d.Status).Scan(
		&out.ID, &out.EvidenceID, &out.Type, &out.Value, &out.Location, &out.DetectionMethod, &out.Status, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert pii detection: %w", err)
	}
	return &out, nil
}

func (r *PIIRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.PIIDetection, error) {
	var d domain.PIIDetection
	err := r.pool.QueryRow(ctx, `
		SELECT id, evidence_id, pii_type, value, location, detection_method, status, created_at
		FROM pii_detections WHERE id = $1
	`, id).Scan(&d.ID, &d.EvidenceID, &d.Type, &d.Value, &d.Location, &d.DetectionMethod, &d.Status, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select pii detection: %w", err)
	}
	return &d, nil
}

// UpdateStatus records the user's accept/reject review decision for one
// detection. Scoped to evidenceID so a caller can't update a detection
// belonging to evidence it doesn't own by guessing an ID.
func (r *PIIRepository) UpdateStatus(ctx context.Context, id, evidenceID uuid.UUID, status domain.PIIStatus) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE pii_detections SET status = $3 WHERE id = $1 AND evidence_id = $2
	`, id, evidenceID, status)
	if err != nil {
		return fmt.Errorf("update pii status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListAcceptedByEvidence returns only the detections the user has
// explicitly accepted for redaction.
func (r *PIIRepository) ListAcceptedByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]domain.PIIDetection, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evidence_id, pii_type, value, location, detection_method, status, created_at
		FROM pii_detections
		WHERE evidence_id = $1 AND status = 'accepted'
		ORDER BY created_at ASC
	`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list accepted pii detections: %w", err)
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
