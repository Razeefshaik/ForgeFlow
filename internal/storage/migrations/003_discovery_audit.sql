CREATE TRIGGER discovery_finished_no_update BEFORE UPDATE ON discovery_runs
WHEN json_extract(OLD.data,'$.status') <> 'RUNNING'
BEGIN SELECT RAISE(ABORT, 'completed discovery runs are immutable'); END;
