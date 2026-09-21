-- Content-safety (NudeNet) scan results, one row per scan. Kept separate
-- from evidence.status (which reflects only the latest verdict) so the
-- underlying signal stays auditable even across re-scans.
CREATE TABLE moderation_results (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id    UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    status         TEXT NOT NULL CHECK (status IN ('safe', 'review', 'sensitive')),
    confidence     DOUBLE PRECISION NOT NULL,
    labels         JSONB NOT NULL DEFAULT '[]'::jsonb,
    bounding_boxes JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_moderation_results_evidence_id ON moderation_results (evidence_id, created_at);
