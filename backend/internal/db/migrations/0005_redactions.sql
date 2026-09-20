-- User-driven PII review and redaction. A redaction always produces a new
-- evidence_files row (kind='redacted') — the original is never modified.

ALTER TABLE pii_detections DROP CONSTRAINT pii_detections_detection_method_check;
ALTER TABLE pii_detections ADD CONSTRAINT pii_detections_detection_method_check
    CHECK (detection_method IN ('regex', 'ai', 'manual'));

CREATE TABLE redactions (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evidence_id       UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    evidence_file_id  UUID NOT NULL REFERENCES evidence_files (id) ON DELETE CASCADE,
    pii_detection_id  UUID NOT NULL REFERENCES pii_detections (id) ON DELETE CASCADE,
    -- Whether the redaction service could actually locate and cover this
    -- PII value in the redacted file (true pixel redaction on images
    -- requires finding it in the OCR word boxes; not every value is found).
    applied           BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_redactions_evidence_id ON redactions (evidence_id);
CREATE INDEX idx_redactions_evidence_file_id ON redactions (evidence_file_id);
