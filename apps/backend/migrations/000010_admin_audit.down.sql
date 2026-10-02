DROP TRIGGER IF EXISTS system_settings_updated_at ON system_settings;
DROP FUNCTION IF EXISTS update_system_settings_updated_at();

DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS audit_logs;
