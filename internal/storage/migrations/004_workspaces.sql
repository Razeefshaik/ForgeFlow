CREATE TABLE workspace_approvals (
 contribution_id TEXT PRIMARY KEY REFERENCES contributions(id),
 token TEXT NOT NULL UNIQUE,
 data TEXT NOT NULL CHECK(json_valid(data))
);
CREATE TRIGGER workspace_approval_no_update BEFORE UPDATE ON workspace_approvals BEGIN SELECT RAISE(ABORT, 'approval history is immutable'); END;
CREATE TRIGGER workspace_approval_no_delete BEFORE DELETE ON workspace_approvals BEGIN SELECT RAISE(ABORT, 'approval history is immutable'); END;
