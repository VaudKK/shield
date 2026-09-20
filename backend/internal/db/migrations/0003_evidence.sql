-- Evidence records, their stored files, and the append-only audit trail.

CREATE TABLE evidence (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'quarantined'
                CHECK (status IN ('quarantined', 'safe', 'review', 'sensitive', 'rejected')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_evidence_owner_id ON evidence (owner_id) WHERE deleted_at IS NULL;

-- Every stored file tied to a piece of evidence: the untouched original,
-- plus derived copies (previews, redactions, exports) added in later phases.
CREATE TABLE evidence_files (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id       UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    kind              TEXT NOT NULL DEFAULT 'original'
                      CHECK (kind IN ('original', 'preview', 'redacted', 'export')),
    original_filename TEXT NOT NULL,
    mime_type         TEXT NOT NULL,
    size_bytes        BIGINT NOT NULL,
    sha256            TEXT NOT NULL,
    storage_key       TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_evidence_files_evidence_id ON evidence_files (evidence_id);

-- Append-only: the application never issues UPDATE or DELETE against this
-- table. Only INSERT and SELECT.
CREATE TABLE audit_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    actor_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_events_evidence_id ON audit_events (evidence_id, created_at);
