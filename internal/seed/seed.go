package seed

import (
	"context"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/estimator"
	"forgeflow/internal/ranking"
	"forgeflow/internal/storage"
	"time"
)

func Load(ctx context.Context, s *storage.Store) error {
	type sample struct {
		repo, title, summary, lang, area, difficulty string
		score                                        float64
		files                                        int
		concurrency, integration                     bool
	}
	samples := []sample{
		{"etcd-io/etcd", "Improve lease retry behavior during leader changes", "Illustrative issue: make retry behavior predictable during leader transitions and cover it with regression tests.", "Go", "distributed-systems", "medium", 94, 8, true, true},
		{"temporalio/temporal", "Preserve cancellation in worker shutdown", "Illustrative issue: ensure pending work respects cancellation while the worker drains.", "Go", "backend", "medium", 91, 6, true, false},
		{"qdrant/qdrant", "Clarify timeout handling in shard transfers", "Illustrative issue: tighten timeout behavior and exercise the transfer boundary.", "Rust", "databases", "medium", 87, 10, true, true},
		{"apache/kafka", "Handle empty offsets in consumer metrics", "Illustrative issue: avoid invalid metric values when no partition offset is available.", "Java", "distributed-systems", "easy", 84, 4, false, true},
		{"mlflow/mlflow", "Validate model artifact paths consistently", "Illustrative issue: normalize model artifact validation and cover malformed paths.", "Python", "ai-infrastructure", "easy", 81, 3, false, false},
		{"go-gitea/gitea", "Return consistent errors for expired tokens", "Illustrative issue: align expired-token responses across API endpoints.", "Go", "backend", "easy", 79, 4, false, false},
	}
	opps := []domain.Opportunity{}
	for i, item := range samples {
		factors := []domain.Factor{
			{Key: "skill", Label: "Skill match", Score: item.score, Weight: 0.2, Reason: "Illustrative language fit for the demo profile."},
			{Key: "health", Label: "Repository health", Score: item.score - 2, Weight: 0.2, Reason: "Demo health signal; live repository analysis has not run."},
			{Key: "clarity", Label: "Issue clarity", Score: item.score + 2, Weight: 0.2, Reason: "Illustrative scope is described with a concrete behavior."},
			{Key: "usefulness", Label: "Contribution usefulness", Score: item.score, Weight: 0.2, Reason: "Illustrative reliability or correctness improvement."},
			{Key: "learning", Label: "Learning & portfolio", Score: item.score, Weight: 0.2, Reason: "Illustrative match to backend and infrastructure interests."},
		}
		r, err := ranking.Score(domain.Quality{Factors: factors})
		if err != nil {
			return err
		}
		opps = append(opps, domain.Opportunity{ID: fmt.Sprintf("demo-%d", i+1), Repository: item.repo, Number: 1001 + i, Title: item.title, Summary: item.summary, Language: item.lang, Domain: item.area, Labels: []string{"help wanted", "bug"}, Difficulty: item.difficulty, MergeLikelihood: "not assessed", Risks: []string{"Seeded example; issue number, scope and repository signals are illustrative.", "Duplicate work and maintainer response have not been checked."}, Ranking: r, Estimate: estimator.Estimate(estimator.Signals{RelevantFiles: item.files, Concurrency: item.concurrency, IntegrationTests: item.integration, ScopeKnown: true}), Demo: true})
	}
	cs := []domain.Contribution{
		{ID: "demo-temporal", OpportunityID: "demo-2", Repository: samples[1].repo, Title: samples[1].title, State: "CODING", Branch: "autopilot/demo-1002", ConfigVersion: 1, UpdatedAt: time.Now().UTC(), Demo: true},
		{ID: "demo-etcd", OpportunityID: "demo-1", Repository: samples[0].repo, Title: samples[0].title, State: "TESTING", Branch: "autopilot/demo-1001", ConfigVersion: 1, UpdatedAt: time.Now().UTC(), Demo: true},
		{ID: "demo-gitea", OpportunityID: "demo-6", Repository: samples[5].repo, Title: samples[5].title, State: "READY", Branch: "autopilot/demo-1006", ConfigVersion: 1, UpdatedAt: time.Now().UTC(), Demo: true},
	}
	return s.SeedOnce(ctx, opps, cs)
}
