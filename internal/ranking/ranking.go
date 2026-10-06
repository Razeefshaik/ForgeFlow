// Package ranking deliberately accepts only quality signals; AI estimates are a separate domain.
package ranking

import (
	"errors"
	"forgeflow/internal/domain"
	"math"
)

func Score(q domain.Quality) (domain.Ranking, error) {
	if len(q.Factors) == 0 {
		return domain.Ranking{}, errors.New("ranking needs explained quality factors")
	}
	total, weights := 0.0, 0.0
	seen := map[string]bool{}
	qualityKeys := map[string]bool{"skill": true, "domain": true, "health": true, "maintainer": true, "clarity": true, "merge": true, "learning": true, "career": true, "usefulness": true, "difficulty": true, "competition": true}
	for _, f := range q.Factors {
		if !qualityKeys[f.Key] || f.Label == "" || f.Reason == "" || seen[f.Key] || math.IsNaN(f.Score) || math.IsInf(f.Score, 0) || f.Score < 0 || f.Score > 100 || math.IsNaN(f.Weight) || math.IsInf(f.Weight, 0) || f.Weight <= 0 {
			return domain.Ranking{}, errors.New("invalid or duplicate ranking factor")
		}
		seen[f.Key] = true
		total += f.Score * f.Weight
		weights += f.Weight
	}
	score := total / weights
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return domain.Ranking{}, errors.New("invalid ranking weights")
	}
	return domain.Ranking{Score: math.Round(score*10) / 10, Factors: append([]domain.Factor(nil), q.Factors...), Version: "quality-v1"}, nil
}
