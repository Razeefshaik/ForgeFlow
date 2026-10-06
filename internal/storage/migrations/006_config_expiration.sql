ALTER TABLE config_proposals ADD COLUMN expires_at TEXT NOT NULL DEFAULT '';
CREATE TABLE config_expirations (version INTEGER PRIMARY KEY REFERENCES config_versions(version), restore_version INTEGER NOT NULL REFERENCES config_versions(version), expires_at TEXT NOT NULL, status TEXT NOT NULL);
CREATE TRIGGER config_expiration_no_delete BEFORE DELETE ON config_expirations BEGIN SELECT RAISE(ABORT,'expiration history is immutable'); END;
