CREATE TABLE executions (contribution_id TEXT PRIMARY KEY REFERENCES contributions(id), data TEXT NOT NULL CHECK(json_valid(data)));
CREATE TABLE agent_runs (id TEXT PRIMARY KEY, contribution_id TEXT NOT NULL REFERENCES contributions(id), data TEXT NOT NULL CHECK(json_valid(data)));
CREATE TABLE test_runs (id TEXT PRIMARY KEY, contribution_id TEXT NOT NULL REFERENCES contributions(id), data TEXT NOT NULL CHECK(json_valid(data)));
CREATE INDEX agent_runs_contribution ON agent_runs(contribution_id);
CREATE INDEX test_runs_contribution ON test_runs(contribution_id);
CREATE TRIGGER test_runs_no_update BEFORE UPDATE ON test_runs BEGIN SELECT RAISE(ABORT, 'test evidence is immutable'); END;
CREATE TRIGGER test_runs_no_delete BEFORE DELETE ON test_runs BEGIN SELECT RAISE(ABORT, 'test evidence is immutable'); END;
CREATE TRIGGER agent_runs_no_delete BEFORE DELETE ON agent_runs BEGIN SELECT RAISE(ABORT, 'agent history is immutable'); END;
CREATE TRIGGER execution_no_delete BEFORE DELETE ON executions BEGIN SELECT RAISE(ABORT, 'execution history is immutable'); END;
