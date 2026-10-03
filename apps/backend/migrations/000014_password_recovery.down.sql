DROP TABLE password_recovery_challenges;
ALTER TABLE users DROP CONSTRAINT users_username_unique;
ALTER TABLE users DROP CONSTRAINT users_username_format;
ALTER TABLE users DROP CONSTRAINT users_auth_version_nonnegative;
ALTER TABLE users DROP COLUMN username;
ALTER TABLE users DROP COLUMN auth_version;
