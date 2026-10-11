package storage

import (
	"context"
	"encoding/json"
)

// ArnyxState projects only bounded current-state fields. Never load activity,
// agent transcripts, test output, diffs or issue evidence into the assistant.
func (s *Store) ArnyxState(ctx context.Context) (map[string]any, error) {
	state := map[string]any{"demo": s.Demo, "limits": "8 recent contributions, 5 ranked opportunities, 8 running agents; counts cover all records"}
	queries := map[string]string{
		"contribution_counts":  `SELECT json_object('state',json_extract(data,'$.state'),'count',count(*)) FROM contributions GROUP BY json_extract(data,'$.state')`,
		"recent_contributions": `SELECT json_object('id',id,'repository',substr(json_extract(data,'$.repository'),1,160),'title',substr(json_extract(data,'$.title'),1,180),'state',json_extract(data,'$.state'),'updated_at',json_extract(data,'$.updated_at')) FROM contributions ORDER BY json_extract(data,'$.updated_at') DESC,id LIMIT 8`,
		"top_opportunities":    `SELECT json_object('id',id,'repository',substr(json_extract(data,'$.repository'),1,160),'title',substr(json_extract(data,'$.title'),1,180),'score',score,'language',json_extract(data,'$.language')) FROM opportunities ORDER BY score DESC,id LIMIT 5`,
		"agent_counts":         `SELECT json_object('status',json_extract(data,'$.status'),'count',count(*)) FROM agent_runs GROUP BY json_extract(data,'$.status')`,
		"running_agents":       `SELECT json_object('contribution_id',contribution_id,'role',json_extract(data,'$.role'),'status',json_extract(data,'$.status'),'model',json_extract(data,'$.model')) FROM agent_runs WHERE json_extract(data,'$.status')='RUNNING' ORDER BY rowid DESC LIMIT 8`,
	}
	for key, query := range queries {
		values, err := listJSON[json.RawMessage](ctx, s.db, query)
		if err != nil {
			return nil, err
		}
		state[key] = values
	}
	return state, nil
}

// ArnyxLookup uses parameters for identifiers; no user/model SQL is accepted.
func (s *Store) ArnyxLookup(ctx context.Context, action, target string) (any, error) {
	var queries []string
	switch action {
	case "findContribution":
		return listJSON[json.RawMessage](ctx, s.db, `SELECT json_object('id',id,'repository',substr(json_extract(data,'$.repository'),1,160),'title',substr(json_extract(data,'$.title'),1,180),'state',json_extract(data,'$.state')) FROM contributions WHERE instr(lower(data ->> '$.repository'),lower(?))>0 OR id=? ORDER BY rowid DESC LIMIT 8`, target, target)
	case "getContribution":
		queries = []string{
			`SELECT json_object('id',id,'repository',substr(data ->> '$.repository',1,160),'title',substr(data ->> '$.title',1,180),'state',data ->> '$.state','message',substr(data ->> '$.message',1,600),'config_version',data ->> '$.config_version','execution_approved',data ->> '$.execution_approved') FROM contributions WHERE id=?`,
			`SELECT json_object('phase',data ->> '$.phase','status',data ->> '$.status','summary',substr(data ->> '$.summary',1,700),'message',substr(data ->> '$.message',1,600),'plan_approved',data ->> '$.plan_approved','review_verdict',data ->> '$.review.verdict','incident_kind',data ->> '$.incident.kind','incident_summary',substr(data ->> '$.incident.summary',1,500),'next_action',data ->> '$.incident.next_action','can_recover',data ->> '$.incident.can_recover','constraints',substr(data ->> '$.constraints',1,1000),'pr_url',data ->> '$.pr_url') FROM executions WHERE contribution_id=?`,
			`SELECT json_object('program',data ->> '$.command.program','exit_code',data ->> '$.exit_code','failure_kind',data ->> '$.failure_kind','finished_at',data ->> '$.finished_at') FROM test_runs WHERE contribution_id=? ORDER BY rowid DESC LIMIT 5`,
		}
	case "getOpportunity":
		queries = []string{`SELECT json_object('id',id,'repository',substr(data ->> '$.repository',1,160),'title',substr(data ->> '$.title',1,180),'summary',substr(data ->> '$.summary',1,900),'language',data ->> '$.language','domain',data ->> '$.domain','score',score,'ranking_factors',json_extract(data,'$.ranking.factors')) FROM opportunities WHERE id=?`}
	default:
		return nil, ErrNotFound
	}
	result := []any{}
	for _, query := range queries {
		values, err := listJSON[json.RawMessage](ctx, s.db, query, target)
		if err != nil {
			return nil, err
		}
		if len(result) == 0 && len(values) == 0 {
			return nil, ErrNotFound
		}
		result = append(result, values)
	}
	return result, nil
}
