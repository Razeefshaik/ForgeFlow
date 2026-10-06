// Package operator exposes a deterministic, allowlisted foundation dispatcher.
// An AI adapter can later choose these same operations; no shell is exposed.
package operator

import (
	"context"
	"fmt"
	"forgeflow/internal/domain"
	"forgeflow/internal/storage"
	"strings"
)

type Reply struct {
	Message  string           `json:"message"`
	Action   string           `json:"action"`
	Proposal *domain.Proposal `json:"proposal,omitempty"`
}
type Service struct{ Store *storage.Store }

func (s Service) Chat(ctx context.Context, message string) (Reply, error) {
	m := strings.ToLower(strings.TrimSpace(message))
	if m == "" || len(message) > 2000 {
		return Reply{}, fmt.Errorf("enter a message of at most 2000 characters")
	}
	c, err := s.Store.CurrentConfig(ctx)
	if err != nil {
		return Reply{}, err
	}
	if strings.Contains(m, "add rust") || strings.Contains(m, "include rust") || strings.Contains(m, "contribute to rust") {
		// Only one supported proposal template. Never silently infer multi-setting changes.
		if m != "add rust" && m != "include rust" && m != "i want to contribute to rust" {
			return Reply{Action: "unsupported", Message: "I can propose “Add Rust” as a single change. Use Configuration for multi-setting requests so you can review every field."}, nil
		}
		if _, ok := c.Config.Profile.Languages["Rust"]; ok {
			return Reply{Action: "getConfig", Message: "Rust is already included in the current profile."}, nil
		}
		c.Config.Profile.Languages["Rust"] = 0.8
		p, err := s.Store.Propose(ctx, c.Version, c.Config, "Operator: add Rust to search languages")
		if err != nil {
			return Reply{}, err
		}
		return Reply{Action: "proposeConfigChange", Message: "Proposed adding Rust with weight 0.8. Review and Apply in Configuration. No settings have changed yet.", Proposal: &p}, nil
	}
	if strings.Contains(m, "language") || strings.Contains(m, "config") {
		return Reply{Action: "getConfig", Message: fmt.Sprintf("Active configuration: v%d. Languages and weights: %v. PR submission and merge automation are disabled.", c.Version, c.Config.Profile.Languages)}, nil
	}
	if strings.Contains(m, "agent") {
		return Reply{Action: "getAgentStatus", Message: "No execution agents are connected. Demo contribution states are illustrative; no Codex session is running."}, nil
	}
	if strings.Contains(m, "discovery") || strings.Contains(m, "scan") {
		if s.Store.Demo {
			return Reply{Action: "getDiscoveryStatus", Message: "Demo mode uses illustrative issues. Start live mode and use Run discovery now in Opportunities for real GitHub observations."}, nil
		}
		runs, e := s.Store.DiscoveryRuns(ctx)
		if e != nil {
			return Reply{}, e
		}
		if len(runs) == 0 {
			return Reply{Action: "getDiscoveryStatus", Message: "No GitHub scans yet. Use Run discovery now in Opportunities. Discovery never starts contribution execution."}, nil
		}
		r := runs[0]
		return Reply{Action: "getDiscoveryStatus", Message: fmt.Sprintf("Latest GitHub scan: %s; %d opportunities retained, %d requests, config v%d. These are bounded observations. See scan history in Opportunities for warnings.", r.Status, r.Accepted, r.Requests, r.ConfigVersion)}, nil
	}
	if strings.Contains(m, "ready") || strings.Contains(m, "contribution") {
		cs, err := s.Store.Contributions(ctx)
		if err != nil {
			return Reply{}, err
		}
		ready := []string{}
		for _, c := range cs {
			if c.State == "READY" {
				ready = append(ready, c.Repository)
			}
		}
		prefix := ""
		if s.Store.Demo {
			prefix = "Demo data: "
		}
		return Reply{Action: "getContribution", Message: fmt.Sprintf("%s%d contributions, %d ready entries (%s). Execution is not available in this foundation.", prefix, len(cs), len(ready), strings.Join(ready, ", "))}, nil
	}
	if strings.Contains(m, "rank") || strings.Contains(m, "score") {
		os, err := s.Store.Opportunities(ctx)
		if err != nil {
			return Reply{}, err
		}
		if len(os) == 0 {
			return Reply{Action: "getOpportunity", Message: "No opportunities have been analyzed. Run GitHub discovery from Opportunities in live mode."}, nil
		}
		for _, o := range os {
			if strings.Contains(m, strings.ToLower(o.Repository)) || strings.Contains(m, o.ID) {
				var reasons []string
				for _, f := range o.Ranking.Factors {
					reasons = append(reasons, fmt.Sprintf("%s %.0f/100: %s", f.Label, f.Score, f.Reason))
				}
				return Reply{Action: "getOpportunity", Message: fmt.Sprintf("%s: %.1f/100. %s Codex effort is excluded from this score.", o.Repository, o.Ranking.Score, strings.Join(reasons, " "))}, nil
			}
		}
		return Reply{Action: "getOpportunity", Message: "Ranking is a weighted average of stored quality components. Codex effort is separate. Include a repository's full name to inspect its stored reasons."}, nil
	}
	return Reply{Action: "unsupported", Message: "Foundation Operator supports current languages, agent status, ready contributions, ranking explanations, and “Add Rust” proposals. AI-backed chat and execution controls are pending."}, nil
}
