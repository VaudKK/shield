-- Safe Disclosure Packages: a user-curated, privacy-reviewed export bundle.
-- The original evidence is never replaced by a disclosure version — a
-- disclosure always produces its own standalone package under exports/.

CREATE TABLE disclosures (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id               UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title                  TEXT NOT NULL,

    include_timeline       BOOLEAN NOT NULL DEFAULT true,
    include_summary        BOOLEAN NOT NULL DEFAULT true,
    include_photos         BOOLEAN NOT NULL DEFAULT true,

    remove_phone_numbers   BOOLEAN NOT NULL DEFAULT true,
    remove_emails          BOOLEAN NOT NULL DEFAULT true,
    remove_id_numbers      BOOLEAN NOT NULL DEFAULT true,
    blur_faces             BOOLEAN NOT NULL DEFAULT false,
    remove_metadata        BOOLEAN NOT NULL DEFAULT true,

    storage_key            TEXT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_disclosures_owner_id ON disclosures (owner_id);

-- Which evidence a package includes. A simple join table; the redaction
-- report itself is generated fresh into the package at creation time
-- rather than duplicated into its own table.
CREATE TABLE disclosure_evidence (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    disclosure_id  UUID NOT NULL REFERENCES disclosures (id) ON DELETE CASCADE,
    evidence_id    UUID NOT NULL REFERENCES evidence (id) ON DELETE CASCADE,
    UNIQUE (disclosure_id, evidence_id)
);

CREATE INDEX idx_disclosure_evidence_disclosure_id ON disclosure_evidence (disclosure_id);
