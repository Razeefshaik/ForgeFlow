package estimator

import "forgeflow/internal/domain"

// Signals are scope observations, never ranking inputs or actual allowance measurements.
type Signals struct {
	RelevantFiles    int
	IntegrationTests bool
	Concurrency      bool
	ScopeKnown       bool
}

func Estimate(s Signals) domain.Estimate {
	level := 1
	if s.RelevantFiles > 5 {
		level++
	}
	if s.RelevantFiles > 15 {
		level++
	}
	if s.IntegrationTests {
		level++
	}
	if s.Concurrency {
		level++
	}
	if level > 5 {
		level = 5
	}
	categories := []string{"VERY_LOW", "LOW", "MEDIUM", "HIGH", "VERY_HIGH"}
	confidence := 0.4
	if s.ScopeKnown {
		confidence = 0.7
	}
	files := s.RelevantFiles
	if files < 1 {
		files = 1
	}
	complexity := "unit tests"
	if s.IntegrationTests {
		complexity = "integration environment required"
	}
	reasons := []string{"Heuristic scope estimate; no plan allowance data is available."}
	if s.Concurrency {
		reasons = append(reasons, "Concurrency increases debugging uncertainty.")
	}
	if !s.ScopeKnown {
		reasons = append(reasons, "Relevant module scope has not been confirmed.")
	}
	return domain.Estimate{Category: categories[level-1], Iterations: [2]int{level, level + 2}, Files: [2]int{files, files + 4}, Context: "module-sized (heuristic)", TestComplexity: complexity, Confidence: confidence, Explanation: reasons}
}
