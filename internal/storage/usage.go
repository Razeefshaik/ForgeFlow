package storage

import "context"

type UsageSummary struct {
	Sessions           int              `json:"sessions"`
	ObservedUsage      map[string]int64 `json:"observed_usage"`
	DurationSeconds    float64          `json:"duration_seconds"`
	AllowanceRemaining *float64         `json:"allowance_remaining"`
	Source             string           `json:"source"`
}

func (s *Store) Usage(ctx context.Context) (UsageSummary, error) {
	v := UsageSummary{ObservedUsage: map[string]int64{}, Source: "all persisted local Codex agent, Arnyx task, and operator events"}
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM agent_runs) +
		(SELECT count(*) FROM arnyx_tasks WHERE json_extract(data,'$.session_id') IS NOT NULL AND json_extract(data,'$.session_id') != '') +
		(SELECT coalesce(sum(CASE WHEN json_type(data,'$.session_ids')='array' THEN json_array_length(data,'$.session_ids') ELSE 0 END),0) FROM events WHERE type='ArnyxReplied'),
		coalesce((SELECT sum(CASE WHEN json_extract(data,'$.finished_at') IS NOT NULL THEN max(0,(julianday(json_extract(data,'$.finished_at'))-julianday(json_extract(data,'$.started_at')))*86400) ELSE 0 END) FROM agent_runs),0) +
		coalesce((SELECT sum(CASE
			WHEN json_extract(data,'$.duration_seconds') IS NOT NULL THEN max(0,CAST(json_extract(data,'$.duration_seconds') AS REAL))
			WHEN json_extract(data,'$.finished_at') IS NOT NULL AND json_extract(data,'$.started_at') IS NOT NULL THEN max(0,(julianday(json_extract(data,'$.finished_at'))-julianday(json_extract(data,'$.started_at')))*86400)
			ELSE 0 END) FROM arnyx_tasks),0) +
		coalesce((SELECT sum(CASE
			WHEN json_extract(data,'$.duration_seconds') IS NOT NULL THEN max(0,CAST(json_extract(data,'$.duration_seconds') AS REAL))
			ELSE 0 END) FROM events WHERE type='ArnyxReplied'),0)`).Scan(&v.Sessions, &v.DurationSeconds)
	if err != nil {
		return v, err
	}
	for _, query := range []string{
		`SELECT u.key,coalesce(sum(CAST(u.value AS INTEGER)),0) FROM agent_runs a,json_each(a.data,'$.usage') u WHERE json_type(a.data,'$.usage')='object' AND u.key IS NOT NULL GROUP BY u.key`,
		`SELECT u.key,coalesce(sum(CAST(u.value AS INTEGER)),0) FROM arnyx_tasks t,json_each(t.data,'$.usage') u WHERE json_type(t.data,'$.usage')='object' AND u.key IS NOT NULL GROUP BY u.key`,
		`SELECT u.key,coalesce(sum(CAST(u.value AS INTEGER)),0) FROM events e,json_each(e.data,'$.usage') u WHERE e.type='ArnyxReplied' AND json_type(e.data,'$.usage')='object' AND u.key IS NOT NULL GROUP BY u.key`,
	} {
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			return v, err
		}
		for rows.Next() {
			var k string
			var n int64
			if err = rows.Scan(&k, &n); err != nil {
				rows.Close()
				return v, err
			}
			v.ObservedUsage[k] += n
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return v, err
		}
		rows.Close()
	}
	return v, nil
}
