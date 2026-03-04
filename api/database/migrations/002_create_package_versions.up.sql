
CREATE TABLE IF NOT EXISTS package_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_id      UUID NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    bottle_s3_key   TEXT NOT NULL,
    sbom_s3_key     TEXT NOT NULL DEFAULT '',
    sha256          TEXT NOT NULL,
    cosign_sig_ref  TEXT NOT NULL DEFAULT '',
    tier            TEXT NOT NULL CHECK (tier IN ('internal', 'external-built', 'external-binary')),
    scan_status     TEXT NOT NULL DEFAULT 'pending' CHECK (scan_status IN ('pending', 'passed', 'failed')),
    approval_status TEXT NOT NULL DEFAULT 'pending' CHECK (approval_status IN ('pending', 'approved', 'denied')),
    approved_by     TEXT,
    approved_at     TIMESTAMPTZ,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (package_id, version)
);

CREATE INDEX idx_package_versions_package_id ON package_versions (package_id);
CREATE INDEX idx_package_versions_approval_status ON package_versions (approval_status);

-- +migrate Down
DROP TABLE IF EXISTS package_versions;
