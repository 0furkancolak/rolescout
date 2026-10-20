-- Latest memory sample of each long-running service, written by the service
-- itself so the web app can show how much RAM the installation uses.
CREATE TABLE service_stats (
    service TEXT PRIMARY KEY CHECK (service ~ '^[a-z][a-z0-9_-]{1,31}$'),
    used_bytes BIGINT NOT NULL,
    limit_bytes BIGINT NOT NULL DEFAULT 0,
    heap_bytes BIGINT NOT NULL DEFAULT 0,
    goroutines INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
