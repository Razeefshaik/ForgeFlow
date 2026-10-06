package analyzer

import (
	"encoding/json"
	"forgeflow/internal/config"
	"forgeflow/internal/domain"
	"forgeflow/internal/github"
	"testing"
	"time"
)

func TestTruncatedTreePreservesUnknownAbsence(t *testing.T) {
	var tree github.Tree
	if err := json.Unmarshal([]byte(`{"truncated":true,"tree":[{"path":"go.mod","type":"blob"},{"path":"pkg/db_test.go","type":"blob"}]}`), &tree); err != nil {
		t.Fatal(err)
	}
	var e domain.Evidence
	InspectTree(tree, &e)
	if e.Tests == nil || !*e.Tests || e.CI != nil || e.Guidelines != nil || e.TreeComplete {
		t.Fatal("truncated tree turned unknown into absent", e)
	}
	tree.Truncated = false
	InspectTree(tree, &e)
	if e.CI == nil || *e.CI || !e.TreeComplete {
		t.Fatal("complete tree absence missing")
	}
}
func TestEvidenceRankingUnknownsCompetitionAndEffortIndependence(t *testing.T) {
	now := time.Now().UTC()
	repo := github.Repository{FullName: "example/backend", Language: "Go", Description: "distributed database backend", Stars: 1000}
	issue := github.Issue{Number: 12, Title: "Fix storage race", Body: "A reproducible backend bug", State: "open", Labels: []github.Label{{Name: "bug"}}, UpdatedAt: now}
	e := domain.Evidence{ObservedAt: now, Warnings: []string{}, CompetingPRs: []string{}}
	o, err := Analyze(repo, issue, e, config.Default(), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Ranking.Factors) != 11 || o.Demo || o.Evidence == nil || o.MergeLikelihood != "unknown" {
		t.Fatal(o)
	}
	for _, f := range o.Ranking.Factors {
		if f.Reason == "" {
			t.Fatal("unexplained factor")
		}
		if (f.Key == "merge" || f.Key == "competition") && f.Score != 50 {
			t.Fatal("missing evidence was treated as certainty", f)
		}
	}
	score := o.Ranking.Score
	o.Estimate.Category = "VERY_HIGH"
	o.Estimate.AllowanceImpact = new(float64)
	*o.Estimate.AllowanceImpact = 99
	if o.Ranking.Score != score {
		t.Fatal("effort affected ranking")
	}
	e.CompetitionChecked = true
	e.CompetingPRs = []string{"https://github.com/example/backend/pull/13"}
	issue.Assignees = []github.User{{Login: "owner"}}
	competing, err := Analyze(repo, issue, e, config.Default(), now)
	if err != nil || competing.Ranking.Score >= score {
		t.Fatal("competition not reflected in quality", err)
	}
}
func TestLabelsDriveConservativeExclusionsAndDifficulty(t *testing.T) {
	c := config.Default()
	i := github.Issue{Body: "backend fix with documentation", Labels: []github.Label{{Name: "bug"}}}
	if Excluded(i, c) {
		t.Fatal("body keyword excluded backend work")
	}
	i.Labels = []github.Label{{Name: "documentation"}}
	if !Excluded(i, c) {
		t.Fatal("documentation-only issue accepted")
	}
	i.Labels = []github.Label{{Name: "good first issue"}}
	if Difficulty(i) != "easy" {
		t.Fatal("beginner difficulty missed")
	}
}
