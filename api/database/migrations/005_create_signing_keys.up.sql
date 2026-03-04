
CREATE TABLE IF NOT EXISTS signing_keys (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id     TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rotated_at TIMESTAMPTZ
);

CREATE INDEX idx_signing_keys_active ON signing_keys (active);

-- +migrate Down
DROP TABLE IF EXISTS signing_keys;
