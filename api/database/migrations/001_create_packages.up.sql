
CREATE TABLE IF NOT EXISTS packages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    tier        TEXT NOT NULL CHECK (tier IN ('internal', 'external-built', 'external-binary')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_packages_name ON packages (name);

-- +migrate Down
DROP TABLE IF EXISTS packages;
