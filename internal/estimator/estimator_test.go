package estimator

import "testing"

func TestHonestScopeEstimate(t *testing.T) {
	low := Estimate(Signals{RelevantFiles: 1, ScopeKnown: true})
	high := Estimate(Signals{RelevantFiles: 30, IntegrationTests: true, Concurrency: true})
	if low.Category != "VERY_LOW" || high.Category != "VERY_HIGH" {
		t.Fatalf("unexpected estimates %+v %+v", low, high)
	}
	if high.AllowanceImpact != nil {
		t.Fatal("invented allowance percentage")
	}
	if high.Confidence >= low.Confidence {
		t.Fatal("unknown scope must lower confidence")
	}
}
