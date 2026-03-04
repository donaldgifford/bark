
CREATE TABLE IF NOT EXISTS approval_records (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    package_version_id UUID NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    action             TEXT NOT NULL CHECK (action IN ('approved', 'denied')),
    actor              TEXT NOT NULL,
    reason             TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_approval_records_package_version_id ON approval_records (package_version_id);

-- +migrate Down
DROP TABLE IF EXISTS approval_records;
