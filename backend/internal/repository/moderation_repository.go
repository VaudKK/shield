package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ModerationRepository struct {
	pool *pgxpool.Pool
}

func NewModerationRepository(pool *pgxpool.Pool) *ModerationRepository {
	return &ModerationRepository{pool: pool}
}

// Create persists one moderation scan result. Results are never updated or
// deleted — a re-scan (e.g. after re-analysis) adds a new row rather than
// overwriting the old one, so the history of what NudeNet reported stays
// intact.
func (r *ModerationRepository) Create(ctx context.Context, m domain.ModerationResult) (*domain.ModerationResult, error) {
	if m.Labels == nil {
		m.Labels = []string{}
	}
	if m.BoundingBox == nil {
		m.BoundingBox = []domain.BoundingBox{}
	}

	labels, err := json.Marshal(m.Labels)
	if err != nil {
		return nil, fmt.Errorf("marshal labels: %w", err)
	}
	boxes, err := json.Marshal(m.BoundingBox)
	if err != nil {
		return nil, fmt.Errorf("marshal bounding boxes: %w", err)
	}

	var out domain.ModerationResult
	var labelsRaw, boxesRaw []byte
	err = r.pool.QueryRow(ctx, `
		INSERT INTO moderation_results (evidence_id, status, confidence, labels, bounding_boxes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, evidence_id, status, confidence, labels, bounding_boxes, created_at
	`, m.EvidenceID, string(m.Status), m.Confidence, labels, boxes).Scan(
		&out.ID, &out.EvidenceID, &out.Status, &out.Confidence, &labelsRaw, &boxesRaw, &out.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert moderation result: %w", err)
	}
	if err := json.Unmarshal(labelsRaw, &out.Labels); err != nil {
		return nil, fmt.Errorf("unmarshal labels: %w", err)
	}
	if err := json.Unmarshal(boxesRaw, &out.BoundingBox); err != nil {
		return nil, fmt.Errorf("unmarshal bounding boxes: %w", err)
	}
	return &out, nil
}

// GetLatestByEvidence returns the most recent moderation scan for a piece
// of evidence, or domain.ErrNotFound if it has never been scanned (e.g. a
// PDF, which content-safety scanning does not apply to).
func (r *ModerationRepository) GetLatestByEvidence(ctx context.Context, evidenceID uuid.UUID) (*domain.ModerationResult, error) {
	var out domain.ModerationResult
	var labelsRaw, boxesRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, evidence_id, status, confidence, labels, bounding_boxes, created_at
		FROM moderation_results
		WHERE evidence_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, evidenceID).Scan(&out.ID, &out.EvidenceID, &out.Status, &out.Confidence, &labelsRaw, &boxesRaw, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select moderation result: %w", err)
	}
	if err := json.Unmarshal(labelsRaw, &out.Labels); err != nil {
		return nil, fmt.Errorf("unmarshal labels: %w", err)
	}
	if err := json.Unmarshal(boxesRaw, &out.BoundingBox); err != nil {
		return nil, fmt.Errorf("unmarshal bounding boxes: %w", err)
	}
	return &out, nil
}
