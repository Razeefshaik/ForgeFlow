package config

import (
	"fmt"
	"forgeflow/internal/domain"
	"math"
	"strings"
)

func Default() domain.Config {
	var c domain.Config
	c.Profile.Languages = map[string]float64{"Go": 1, "Java": 0.8, "Python": 0.7}
	c.Profile.Domains = map[string]float64{"distributed-systems": 1, "backend": 1, "databases": 0.9, "ai-infrastructure": 0.8}
	c.Profile.Exclude = []string{"frontend-only", "documentation-only"}
	c.Repositories.RecentActivityDays = 60
	c.Issues.PreferredLabels = []string{"good first issue", "help wanted", "bug", "enhancement"}
	c.Issues.Difficulty = []string{"easy", "medium"}
	c.Agents.MaxScouts = 4
	c.Agents.MaxContributors = 2
	c.Agents.MaxReviewers = 2
	c.Codex.MaxFixIterations = 5
	c.Codex.MaxReviewCycles = 3
	return c
}
func Validate(c domain.Config) error {
	if c.Contributions.AutoCreatePR || c.Contributions.AutoMerge {
		return fmt.Errorf("automatic PR submission and merging are forbidden")
	}
	for name, values := range map[string]map[string]float64{"languages": c.Profile.Languages, "domains": c.Profile.Domains} {
		if len(values) == 0 || len(values) > 50 {
			return fmt.Errorf("%s must contain 1–50 entries", name)
		}
		for key, weight := range values {
			if strings.TrimSpace(key) == "" || len(key) > 80 || math.IsNaN(weight) || math.IsInf(weight, 0) || weight <= 0 || weight > 1 {
				return fmt.Errorf("invalid %s weight for %q: expected (0,1]", name, key)
			}
		}
	}
	if c.Repositories.MinStars < 0 || c.Repositories.RecentActivityDays < 1 || c.Repositories.RecentActivityDays > 3650 {
		return fmt.Errorf("invalid repository filters")
	}
	if c.Agents.MaxScouts < 1 || c.Agents.MaxScouts > 16 || c.Agents.MaxContributors < 1 || c.Agents.MaxContributors > 8 || c.Agents.MaxReviewers < 1 || c.Agents.MaxReviewers > 8 {
		return fmt.Errorf("invalid agent concurrency limits")
	}
	if c.Codex.MaxFixIterations < 1 || c.Codex.MaxFixIterations > 20 || c.Codex.MaxReviewCycles < 1 || c.Codex.MaxReviewCycles > 10 {
		return fmt.Errorf("invalid retry limits")
	}
	if len(c.Issues.Difficulty) == 0 {
		return fmt.Errorf("at least one difficulty is required")
	}
	for _, d := range c.Issues.Difficulty {
		if d != "easy" && d != "medium" && d != "hard" {
			return fmt.Errorf("invalid difficulty %q", d)
		}
	}
	for _, list := range [][]string{c.Profile.Exclude, c.Issues.PreferredLabels} {
		if len(list) > 50 {
			return fmt.Errorf("too many filter entries")
		}
		for _, item := range list {
			if strings.TrimSpace(item) == "" || len(item) > 100 {
				return fmt.Errorf("invalid filter entry")
			}
		}
	}
	return nil
}
