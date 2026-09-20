-- OCR/AI analysis results, the timeline they produce, and detected PII,
-- pending the user's review before any redaction (a later phase).

CREATE TABLE evidence_analysis (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id  UUID NOT NULL UNIQUE REFERENCES evidence (id) ON DELETE CASCADE,
    ocr_text     TEXT NOT NULL DEFAULT '',
    summary      TEXT NOT NULL DEFAULT '',
    gaps         JSONB NOT NULL DEFAULT '[]'::jsonb,
    model        TEXT NOT NULL DEFAULT '',
    analyzed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE timeline_events (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id  UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    -- Raw, possibly-partial date text as extracted (e.g. "2026-09-12" or
    -- "unknown"). Not parsed into a DATE column since evidence dates are
    -- often incomplete or in conflict, and the UI needs to show that as-is.
    event_date   TEXT NOT NULL,
    description  TEXT NOT NULL,
    source       TEXT NOT NULL DEFAULT 'ai' CHECK (source IN ('ai', 'user')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_timeline_events_evidence_id ON timeline_events (evidence_id);

CREATE TABLE pii_detections (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id       UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    pii_type          TEXT NOT NULL,
    value             TEXT NOT NULL,
    location          TEXT NOT NULL DEFAULT '',
    detection_method  TEXT NOT NULL CHECK (detection_method IN ('regex', 'ai')),
    status            TEXT NOT NULL DEFAULT 'detected' CHECK (status IN ('detected', 'accepted', 'rejected')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_pii_detections_evidence_id ON pii_detections (evidence_id);
