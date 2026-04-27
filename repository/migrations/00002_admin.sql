-- +goose Up

ALTER TABLE user_accounts ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS error_logs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id       UUID        REFERENCES jobs(id) ON DELETE SET NULL,
    user_id      UUID        REFERENCES user_accounts(id) ON DELETE SET NULL,
    error_type   TEXT        NOT NULL,
    error_message TEXT       NOT NULL,
    url          TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS error_logs_created_at_idx ON error_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS error_logs_type_idx       ON error_logs(error_type);

-- +goose Down

ALTER TABLE user_accounts DROP COLUMN IF EXISTS is_admin;
DROP TABLE IF EXISTS error_logs;
