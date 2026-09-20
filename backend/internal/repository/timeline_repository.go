package repository

import (
	"context"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TimelineRepository struct {
	pool *pgxpool.Pool
}

func NewTimelineRepository(pool *pgxpool.Pool) *TimelineRepository {
	return &TimelineRepository{pool: pool}
}

// ReplaceAIEventsForEvidence swaps out all AI-sourced timeline events for
// this evidence with a fresh set, so re-running analysis doesn't
// accumulate duplicates. User-added events (a later phase) are untouched.
func (r *TimelineRepository) ReplaceAIEventsForEvidence(ctx context.Context, evidenceID uuid.UUID, events []domain.TimelineEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM timeline_events WHERE evidence_id = $1 AND source = 'ai'`, evidenceID,
	); err != nil {
		return fmt.Errorf("delete old ai timeline events: %w", err)
	}

	for _, e := range events {
		if _, err := tx.Exec(ctx, `
			INSERT INTO timeline_events (evidence_id, event_date, description, source)
			VALUES ($1, $2, $3, $4)
		`, evidenceID, e.EventDate, e.Description, e.Source); err != nil {
			return fmt.Errorf("insert timeline event: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *TimelineRepository) ListByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]domain.TimelineEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evidence_id, event_date, description, source, created_at
		FROM timeline_events
		WHERE evidence_id = $1
		ORDER BY event_date ASC, created_at ASC
	`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list timeline events: %w", err)
	}
	defer rows.Close()

	var out []domain.TimelineEvent
	for rows.Next() {
		var e domain.TimelineEvent
		if err := rows.Scan(&e.ID, &e.EvidenceID, &e.EventDate, &e.Description, &e.Source, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan timeline event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
