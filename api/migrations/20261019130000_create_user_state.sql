-- Per-user, per-workspace state the web app keeps on the server: where the
-- user left off in the job browser and their autofill details.
CREATE TABLE user_state (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    key TEXT NOT NULL CHECK (key IN ('jobs_view', 'autofill')),
    value JSONB NOT NULL CHECK (pg_column_size(value) <= 65536),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, workspace_id, key)
);
