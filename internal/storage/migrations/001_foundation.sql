CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE config_versions (
 version INTEGER PRIMARY KEY AUTOINCREMENT,
 config TEXT NOT NULL CHECK(json_valid(config)),
 actor TEXT NOT NULL, reason TEXT NOT NULL, created_at TEXT NOT NULL
);
CREATE TABLE config_proposals (
 id TEXT PRIMARY KEY, base_version INTEGER NOT NULL REFERENCES config_versions(version),
 config TEXT NOT NULL CHECK(json_valid(config)), reason TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('PENDING','APPLIED','CANCELLED')), created_at TEXT NOT NULL
);
CREATE TABLE opportunities (
 id TEXT PRIMARY KEY, data TEXT NOT NULL CHECK(json_valid(data)), score REAL NOT NULL CHECK(score BETWEEN 0 AND 100)
);
CREATE TABLE contributions (
 id TEXT PRIMARY KEY, opportunity_id TEXT NOT NULL REFERENCES opportunities(id),
 data TEXT NOT NULL CHECK(json_valid(data))
);
CREATE TABLE events (
 id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL, entity_id TEXT NOT NULL,
 actor TEXT NOT NULL, message TEXT NOT NULL, data TEXT NOT NULL CHECK(json_valid(data)),
 created_at TEXT NOT NULL, demo INTEGER NOT NULL CHECK(demo IN (0,1))
);
CREATE INDEX events_entity_cursor ON events(entity_id,id);
CREATE INDEX opportunities_score ON opportunities(score DESC,id);
CREATE TRIGGER events_no_update BEFORE UPDATE ON events BEGIN SELECT RAISE(ABORT, 'audit events are immutable'); END;
CREATE TRIGGER events_no_delete BEFORE DELETE ON events BEGIN SELECT RAISE(ABORT, 'audit events are immutable'); END;
CREATE TRIGGER config_no_update BEFORE UPDATE ON config_versions BEGIN SELECT RAISE(ABORT, 'config versions are immutable'); END;
CREATE TRIGGER config_no_delete BEFORE DELETE ON config_versions BEGIN SELECT RAISE(ABORT, 'config versions are immutable'); END;

