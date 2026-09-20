package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VaudKK/shield/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepository struct {
	pool *pgxpool.Pool
}

func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{pool: pool}
}

// Record appends an audit event. There is deliberately no Update or Delete
// method on this repository: the audit trail is append-only.
func (r *AuditRepository) Record(ctx context.Context, evidenceID uuid.UUID, eventType string, actorID *uuid.UUID, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_events (evidence_id, event_type, actor_id, metadata)
		VALUES ($1, $2, $3, $4)
	`, evidenceID, eventType, actorID, payload)
	if err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (r *AuditRepository) ListByEvidence(ctx context.Context, evidenceID uuid.UUID) ([]domain.AuditEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, evidence_id, event_type, actor_id, metadata, created_at
		FROM audit_events
		WHERE evidence_id = $1
		ORDER BY created_at ASC
	`, evidenceID)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var payload []byte
		if err := rows.Scan(&e.ID, &e.EvidenceID, &e.EventType, &e.ActorID, &payload, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		if err := json.Unmarshal(payload, &e.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal audit metadata: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
