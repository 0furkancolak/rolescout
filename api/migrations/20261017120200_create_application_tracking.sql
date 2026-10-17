CREATE TABLE saved_jobs (
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs (id) ON DELETE RESTRICT,
    saved_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, job_id)
);
CREATE INDEX saved_jobs_workspace_created_idx ON saved_jobs (workspace_id, created_at DESC);

CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES jobs (id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'planned'
        CHECK (status IN ('planned', 'applied', 'interviewing', 'offer', 'accepted', 'rejected', 'withdrawn')),
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, job_id),
    UNIQUE (workspace_id, id)
);
CREATE INDEX applications_workspace_status_idx ON applications (workspace_id, status, updated_at DESC);
CREATE TRIGGER applications_set_updated_at BEFORE UPDATE ON applications
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();

CREATE TABLE application_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    application_id UUID NOT NULL,
    attempt_number INTEGER NOT NULL CHECK (attempt_number > 0),
    applied_at TIMESTAMPTZ NOT NULL,
    channel TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, attempt_number),
    UNIQUE (application_id, id),
    FOREIGN KEY (workspace_id, application_id)
        REFERENCES applications (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE application_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    application_id UUID NOT NULL,
    attempt_id UUID,
    type TEXT NOT NULL
        CHECK (type IN ('created', 'status_changed', 'attempt_added', 'interview', 'follow_up', 'deadline')),
    from_status TEXT,
    to_status TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    details JSONB NOT NULL DEFAULT '{}',
    created_by UUID REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (type <> 'status_changed' OR to_status IS NOT NULL),
    FOREIGN KEY (workspace_id, application_id)
        REFERENCES applications (workspace_id, id) ON DELETE CASCADE,
    -- An event's attempt must belong to the same application.
    FOREIGN KEY (application_id, attempt_id)
        REFERENCES application_attempts (application_id, id) ON DELETE CASCADE
);
CREATE INDEX application_events_application_idx ON application_events (application_id, occurred_at, id);
-- Upcoming interviews, follow-ups and deadlines per workspace.
CREATE INDEX application_events_upcoming_idx ON application_events (workspace_id, occurred_at)
    WHERE type IN ('interview', 'follow_up', 'deadline');

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (btrim(name) <> '' AND length(name) <= 50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, id)
);
CREATE UNIQUE INDEX tags_workspace_name_key ON tags (workspace_id, lower(name));

CREATE TABLE application_tags (
    workspace_id UUID NOT NULL,
    application_id UUID NOT NULL,
    tag_id UUID NOT NULL,
    PRIMARY KEY (application_id, tag_id),
    FOREIGN KEY (workspace_id, application_id)
        REFERENCES applications (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, tag_id)
        REFERENCES tags (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX application_tags_tag_idx ON application_tags (tag_id);

CREATE TABLE notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    application_id UUID NOT NULL,
    author_id UUID REFERENCES users (id) ON DELETE SET NULL,
    body TEXT NOT NULL CHECK (btrim(body) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (workspace_id, application_id)
        REFERENCES applications (workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX notes_application_idx ON notes (application_id, created_at);
CREATE TRIGGER notes_set_updated_at BEFORE UPDATE ON notes
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();
