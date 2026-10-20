-- Installation-wide settings an administrator edits in the web app (the AI
-- connection, including its API key). A row overrides the matching environment
-- variable; deleting it falls back to the environment.
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{1,63}$'),
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
