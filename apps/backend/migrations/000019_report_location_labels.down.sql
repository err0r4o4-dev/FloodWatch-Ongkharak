ALTER TABLE community_reports
    DROP CONSTRAINT community_reports_location_name_valid,
    DROP CONSTRAINT community_reports_location_note_valid,
    DROP COLUMN location_name,
    DROP COLUMN location_note;
