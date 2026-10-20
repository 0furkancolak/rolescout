-- Automatic runs of a profile can be switched off without deleting it.
ALTER TABLE research_profiles ADD COLUMN schedule_paused BOOLEAN NOT NULL DEFAULT FALSE;
