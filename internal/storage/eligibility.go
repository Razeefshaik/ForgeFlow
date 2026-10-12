package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"forgeflow/internal/domain"
	"strings"
	"time"
)

// RepositoryExclusions uses discovery-time assessments of repository contents.
// It does not infer eligibility from abandoned or failed contributions.
func (s *Store) RepositoryExclusions(ctx context.Context) ([]domain.RepositoryExclusion, error) {
	return repositoryExclusions(ctx, s.db, s.AssessmentProfile)
}

type eligibilityReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func repositoryExclusions(ctx context.Context, db eligibilityReader, profile string) ([]domain.RepositoryExclusion, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM repository_assessments WHERE json_extract(data,'$.status')!='eligible' OR (?!='' AND json_extract(data,'$.profile')!=?) ORDER BY repository`, profile, profile)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []domain.RepositoryExclusion{}
	for rows.Next() {
		var raw string
		var a domain.RepositoryAssessment
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			return nil, err
		}
		if profile != "" && a.Profile != profile {
			a.Status = "needs_assessment"
			a.Findings = []domain.PrerequisiteFinding{{Kind: "needs_assessment", Source: "runner", Reason: "Runner capabilities changed since this assessment; run discovery again before ranking."}}
		}
		v := domain.RepositoryExclusion{Repository: a.Repository, HeadSHA: a.HeadSHA, Status: a.Status, DetectedAt: a.CheckedAt, Checks: a.Findings, NextAction: "Inspect the prerequisite evidence and supported runner capabilities, then run discovery again to reassess the repository."}
		if len(a.Findings) > 0 {
			v.Kind = a.Findings[0].Kind
			v.Reason = a.Findings[0].Reason
		}
		all = append(all, v)
	}
	return all, rows.Err()
}

func repositoryEligibility(repo string, exclusions []domain.RepositoryExclusion) error {
	for _, v := range exclusions {
		if strings.EqualFold(v.Repository, repo) {
			return fmt.Errorf("repository excluded by prerequisite screening (%s): %s", v.Status, v.Reason)
		}
	}
	return nil
}

func (s *Store) SaveRepositoryAssessment(ctx context.Context, a domain.RepositoryAssessment) error {
	return s.transact(ctx, func(tx *sql.Tx) error {
		if err := saveRepositoryAssessment(ctx, tx, a); err != nil {
			return err
		}
		return s.event(ctx, tx, "RepositoryAssessed", a.Repository, "prerequisites", "Repository prerequisite evidence saved", a)
	})
}

func saveRepositoryAssessment(ctx context.Context, tx *sql.Tx, a domain.RepositoryAssessment) error {
	if a.Repository == "" || a.Profile == "" || a.CheckedAt.IsZero() || (a.Status != "eligible" && a.Status != "excluded" && a.Status != "needs_assessment") {
		return fmt.Errorf("invalid repository prerequisite assessment")
	}
	if a.Status == "eligible" && len(a.Findings) > 0 || a.Status != "eligible" && len(a.Findings) == 0 {
		return fmt.Errorf("repository assessment status must match its findings")
	}
	var checked string
	err := tx.QueryRowContext(ctx, "SELECT json_extract(data,'$.checked_at') FROM repository_assessments WHERE repository=?", a.Repository).Scan(&checked)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		old, e := time.Parse(time.RFC3339Nano, checked)
		if e != nil {
			return e
		}
		if a.CheckedAt.Before(old) {
			return nil
		}
	}
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO repository_assessments VALUES (?,?) ON CONFLICT(repository) DO UPDATE SET data=excluded.data", a.Repository, string(b))
	return err
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
