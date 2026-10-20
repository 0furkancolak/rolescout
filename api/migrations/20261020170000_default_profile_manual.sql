-- A new workspace's starter profile is a suggestion, not a standing order: it
-- only runs when someone presses run, until auto-run is switched on.
CREATE OR REPLACE FUNCTION rolescout_default_research_profile() RETURNS trigger AS $$
BEGIN
    INSERT INTO research_profiles (workspace_id, name, title_keywords, technologies, schedule_paused)
    VALUES (NEW.id, 'Full Stack Developer', ARRAY['full stack', 'fullstack', 'full-stack'],
            ARRAY['react', 'typescript', 'node.js', 'javascript', 'postgresql'], TRUE);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- The starter profiles that exist already and were never edited stop running by themselves too.
UPDATE research_profiles SET schedule_paused = TRUE
WHERE name = 'Full Stack Developer' AND updated_at = created_at AND NOT schedule_paused;
