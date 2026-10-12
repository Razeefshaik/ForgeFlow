package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"forgeflow/internal/domain"
)

const searchPageSize = 25
const maxSearchPage = 40

func (s *Service) searchScope(cfg domain.Config) string {
	v := struct {
		Languages            map[string]float64
		Labels               []string
		MinStars, RecentDays int
		Difficulty, Exclude  []string
		Runner               string
	}{cfg.Profile.Languages, cfg.Issues.PreferredLabels, cfg.Repositories.MinStars, cfg.Repositories.RecentActivityDays, cfg.Issues.Difficulty, cfg.Profile.Exclude, s.PrerequisiteProfile.Fingerprint()}
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func nextPosition(page, index, total, count int, incomplete bool) domain.DiscoveryCursor {
	if index+1 < count {
		return domain.DiscoveryCursor{Page: page, Offset: index + 1}
	}
	if page >= maxSearchPage || !incomplete && page*searchPageSize >= total {
		return domain.DiscoveryCursor{Page: 1}
	}
	return domain.DiscoveryCursor{Page: page + 1}
}
