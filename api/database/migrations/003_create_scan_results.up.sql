
CREATE TABLE IF NOT EXISTS scan_results (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_version_id UUID NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    scanner            TEXT NOT NULL,
    result_s3_key      TEXT NOT NULL,
    passed             BOOLEAN NOT NULL,
    summary_json       JSONB,
    scanned_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scan_results_package_version_id ON scan_results (package_version_id);

-- +migrate Down
DROP TABLE IF EXISTS scan_results;
