CREATE TABLE repository_assessments (
 repository TEXT PRIMARY KEY COLLATE NOCASE,
 data TEXT NOT NULL CHECK(json_valid(data))
);
CREATE TRIGGER assessments_no_delete BEFORE DELETE ON repository_assessments BEGIN SELECT RAISE(ABORT, 'repository assessment history is retained in audit events'); END;
