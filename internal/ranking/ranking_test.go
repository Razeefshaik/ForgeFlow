package ranking

import (
	"forgeflow/internal/domain"
	"math"
	"testing"
)

func TestCodexEffortNeverAffectsCanonicalScore(t *testing.T) {
	o := domain.Opportunity{Ranking: domain.Ranking{Factors: []domain.Factor{{Key: "health", Label: "Repository health", Score: 94, Weight: 1, Reason: "Recent maintained releases"}}}}
	before, err := Score(domain.Quality{Factors: o.Ranking.Factors})
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range []string{"VERY_LOW", "LOW", "MEDIUM", "HIGH", "VERY_HIGH"} {
		o.Estimate = domain.Estimate{Category: category, Confidence: 0.99, Files: [2]int{100, 1000}, Iterations: [2]int{20, 100}}
		after, err := Score(domain.Quality{Factors: o.Ranking.Factors})
		if err != nil {
			t.Fatal(err)
		}
		if after.Score != before.Score {
			t.Fatalf("effort %s changed score %v → %v", category, before.Score, after.Score)
		}
	}
}
func TestWeightedQualityScore(t *testing.T) {
	r, e := Score(domain.Quality{Factors: []domain.Factor{{Key: "health", Label: "Health", Score: 100, Weight: 3, Reason: "A"}, {Key: "clarity", Label: "Clarity", Score: 40, Weight: 1, Reason: "B"}}})
	if e != nil || r.Score != 85 {
		t.Fatalf("got %+v %v", r, e)
	}
}
func TestRejectsUnexplainedAndInvalidFactors(t *testing.T) {
	for _, f := range []domain.Factor{{Key: "health", Label: "Health", Score: 101, Weight: 1, Reason: "x"}, {Key: "health", Label: "Health", Score: math.NaN(), Weight: 1, Reason: "x"}, {Key: "health", Label: "Health", Score: 50, Weight: 1}, {Key: "codex_effort", Label: "Codex effort", Score: 50, Weight: 1, Reason: "Must never rank by effort"}} {
		if _, e := Score(domain.Quality{Factors: []domain.Factor{f}}); e == nil {
			t.Fatal("accepted invalid factor")
		}
	}
}
