package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"forgeflow/internal/domain"
	"strings"
)

// RepositoryExclusions projects only the current diagnosis, never full execution
// records or event history. Resolved records therefore restore eligibility.
func (s *Store) RepositoryExclusions(ctx context.Context) ([]domain.RepositoryExclusion, error) {
	return repositoryExclusions(ctx, s.db)
}

type eligibilityReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func repositoryExclusions(ctx context.Context, db eligibilityReader) ([]domain.RepositoryExclusion, error) {
	rows, err := db.QueryContext(ctx, `SELECT json_object(
		'repository',json_extract(c.data,'$.repository'),
		'contribution_id',c.id,
		'kind',json_extract(e.data,'$.incident.kind'),
		'reason',json_extract(e.data,'$.incident.summary'),
		'next_action',json_extract(e.data,'$.incident.next_action'),
		'detected_at',json_extract(e.data,'$.incident.detected_at'))
		FROM executions e JOIN contributions c ON c.id=e.contribution_id
		WHERE json_extract(e.data,'$.status') IN ('NEEDS_INPUT','BLOCKED','PAUSED','ABANDONED','FAILED')
		AND COALESCE(json_extract(e.data,'$.incident.status'),'') != 'resolved'
		AND (json_extract(e.data,'$.incident.kind') IN ('environment_blocked','toolchain_mismatch')
		 OR (json_extract(e.data,'$.incident.source')='environment'
		 AND json_extract(e.data,'$.incident.kind') IN ('verification_timeout','missing_executable')))
		ORDER BY lower(json_extract(c.data,'$.repository')),c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []domain.RepositoryExclusion{}
	for rows.Next() {
		var raw string
		var v domain.RepositoryExclusion
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		all = append(all, v)
	}
	return all, rows.Err()
}

func repositoryEligibility(repo string, exclusions []domain.RepositoryExclusion) error {
	for _, v := range exclusions {
		if strings.EqualFold(v.Repository, repo) {
			return fmt.Errorf("repository excluded until its environment prerequisites are resolved in %s: %s", v.ContributionID, v.Reason)
		}
	}
	return nil
}

// RequireRepositoryEligible guards new selections; existing contributions still
// use their explicit recovery workflow.
func (s *Store) RequireRepositoryEligible(ctx context.Context, repo string) error {
	all, err := s.RepositoryExclusions(ctx)
	if err != nil {
		return err
	}
	return repositoryEligibility(repo, all)
}
