package discovery

import (
	"context"
	"testing"
)

func TestNewRepositoryExcludedBeforeIssueRankingWithNoContributionHistory(t *testing.T) {
	for _, tc := range []struct{ name, body, kind string }{
		{"signing", "Contributors must supply an Ed25519 signed receipt.", "trusted_signing"},
		{"service", "Verification requires a running private server and credentials.", "private_credentials"},
		{"privileged-docker", "Required verification must run docker compose up.", "nested_docker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, calls := fixtureFiles(t, map[string]string{"CONTRIBUTING.md": tc.body, "go.mod": "module example/backend\ngo 1.27.1"})
			s, store := setup(t, server, false)
			ctx := context.Background()
			if _, err := s.Start(); err != nil {
				t.Fatal(err)
			}
			s.Wait()
			runs, err := store.DiscoveryRuns(ctx)
			if err != nil || len(runs) != 1 || runs[0].Accepted != 0 || len(runs[0].ExcludedRepositories) != 1 {
				t.Fatal(runs, err)
			}
			x := runs[0].ExcludedRepositories[0]
			found := false
			for _, f := range x.Checks {
				if f.Kind == tc.kind {
					found = true
				}
			}
			if !found {
				t.Fatal(x)
			}
			cfg, err := store.CurrentConfig(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if calls.Load() != int64(len(cfg.Config.Profile.Languages)+5) {
				t.Fatal("rejected repository consumed issue/release/PR enrichment", calls.Load())
			}
			cs, err := store.Contributions(ctx)
			if err != nil || len(cs) != 0 {
				t.Fatal("screening created a contribution", cs, err)
			}
			opps, err := store.Opportunities(ctx)
			if err != nil || len(opps) != 0 {
				t.Fatal("rejected repository was ranked", opps, err)
			}
			events, err := store.Events(ctx, 0, "", 100)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range events {
				if e.Type == "IssueRanked" || e.Type == "AgentRunUpdated" {
					t.Fatal("rejected repository consumed ranking or agent execution", e)
				}
			}
		})
	}
}
