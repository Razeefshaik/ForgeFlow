CREATE TRIGGER agent_runs_finished_no_update BEFORE UPDATE ON agent_runs
WHEN json_extract(OLD.data, '$.status') != 'RUNNING'
BEGIN SELECT RAISE(ABORT, 'completed agent evidence is immutable'); END;
