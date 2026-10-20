-- What the AI knows about the person behind a workspace: a note about the work
-- they look for and, when uploaded, the CV (its text and what was read from it).
CREATE TABLE workspace_context (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    about TEXT NOT NULL DEFAULT '' CHECK (char_length(about) <= 4000),
    cv_filename TEXT NOT NULL DEFAULT '' CHECK (char_length(cv_filename) <= 255),
    cv_text TEXT NOT NULL DEFAULT '' CHECK (char_length(cv_text) <= 60000),
    cv_skills TEXT[] NOT NULL DEFAULT '{}',
    cv_titles TEXT[] NOT NULL DEFAULT '{}',
    cv_seniority TEXT NOT NULL DEFAULT '' CHECK (cv_seniority IN ('', 'intern', 'junior', 'mid', 'senior', 'lead')),
    cv_years INTEGER CHECK (cv_years IS NULL OR cv_years BETWEEN 0 AND 60),
    cv_summary TEXT NOT NULL DEFAULT '' CHECK (char_length(cv_summary) <= 2000),
    cv_uploaded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TRIGGER workspace_context_set_updated_at BEFORE UPDATE ON workspace_context
    FOR EACH ROW EXECUTE FUNCTION rolescout_set_updated_at();
