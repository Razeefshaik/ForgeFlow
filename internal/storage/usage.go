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
	v := UsageSummary{ObservedUsage: map[string]int64{}, Source: "all persisted local Codex agent events"}
	err := s.db.QueryRowContext(ctx, `SELECT count(*),coalesce(sum(CASE WHEN json_extract(data,'$.finished_at') IS NOT NULL THEN max(0,(julianday(json_extract(data,'$.finished_at'))-julianday(json_extract(data,'$.started_at')))*86400) ELSE 0 END),0) FROM agent_runs`).Scan(&v.Sessions, &v.DurationSeconds)
	if err != nil {
		return v, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.key,sum(CAST(u.value AS INTEGER)) FROM agent_runs a,json_each(a.data,'$.usage') u GROUP BY u.key`)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int64
		if err = rows.Scan(&k, &n); err != nil {
			return v, err
		}
		v.ObservedUsage[k] = n
	}
	return v, rows.Err()
}
