ALTER TABLE community_reports
    ADD COLUMN location_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN location_note TEXT NOT NULL DEFAULT '',
    ADD CONSTRAINT community_reports_location_name_valid CHECK (char_length(location_name) <= 200),
    ADD CONSTRAINT community_reports_location_note_valid CHECK (char_length(location_note) <= 500);
