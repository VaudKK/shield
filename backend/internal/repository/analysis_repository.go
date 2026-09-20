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

type AnalysisRepository struct {
	pool *pgxpool.Pool
}

func NewAnalysisRepository(pool *pgxpool.Pool) *AnalysisRepository {
	return &AnalysisRepository{pool: pool}
}

// Upsert replaces any existing analysis for the evidence, so re-running
// analysis doesn't accumulate stale rows.
func (r *AnalysisRepository) Upsert(ctx context.Context, a domain.Analysis) (*domain.Analysis, error) {
	gapsJSON, err := json.Marshal(a.Gaps)
	if err != nil {
		return nil, fmt.Errorf("marshal gaps: %w", err)
	}

	var out domain.Analysis
	var gapsRaw []byte
	err = r.pool.QueryRow(ctx, `
		INSERT INTO evidence_analysis (evidence_id, ocr_text, summary, gaps, model)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (evidence_id) DO UPDATE
			SET ocr_text = EXCLUDED.ocr_text,
				summary = EXCLUDED.summary,
				gaps = EXCLUDED.gaps,
				model = EXCLUDED.model,
				analyzed_at = now()
		RETURNING id, evidence_id, ocr_text, summary, gaps, model, analyzed_at
	`, a.EvidenceID, a.OCRText, a.Summary, gapsJSON, a.Model).Scan(
		&out.ID, &out.EvidenceID, &out.OCRText, &out.Summary, &gapsRaw, &out.Model, &out.AnalyzedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert evidence analysis: %w", err)
	}
	if err := json.Unmarshal(gapsRaw, &out.Gaps); err != nil {
		return nil, fmt.Errorf("unmarshal gaps: %w", err)
	}
	return &out, nil
}

func (r *AnalysisRepository) GetByEvidenceID(ctx context.Context, evidenceID uuid.UUID) (*domain.Analysis, error) {
	var out domain.Analysis
	var gapsRaw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, evidence_id, ocr_text, summary, gaps, model, analyzed_at
		FROM evidence_analysis WHERE evidence_id = $1
	`, evidenceID).Scan(
		&out.ID, &out.EvidenceID, &out.OCRText, &out.Summary, &gapsRaw, &out.Model, &out.AnalyzedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select evidence analysis: %w", err)
	}
	if err := json.Unmarshal(gapsRaw, &out.Gaps); err != nil {
		return nil, fmt.Errorf("unmarshal gaps: %w", err)
	}
	return &out, nil
}
