CREATE TABLE discovery_runs (
 id TEXT PRIMARY KEY, data TEXT NOT NULL CHECK(json_valid(data)), started_at TEXT NOT NULL
);
CREATE TRIGGER discovery_no_delete BEFORE DELETE ON discovery_runs BEGIN SELECT RAISE(ABORT, 'discovery history is retained'); END;
